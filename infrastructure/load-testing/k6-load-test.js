import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const errorRate = new Rate("errors");
const loginDuration = new Trend("login_duration");
const orderDuration = new Trend("order_duration");

export const options = {
  stages: [
    { duration: "1m",  target: 50  },  // Ramp up
    { duration: "3m",  target: 200 },  // Sustained load
    { duration: "1m",  target: 500 },  // Spike
    { duration: "2m",  target: 200 },  // Cool down
    { duration: "1m",  target: 0   },  // Ramp down
  ],
  thresholds: {
    // SLO: 99.9% of requests under 200ms
    http_req_duration: ["p(99.9)<200"],
    // SLO: Error rate < 0.1%
    errors: ["rate<0.001"],
    // Custom metrics
    login_duration:  ["p(95)<150"],
    order_duration:  ["p(95)<300"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8000";

// Test data
const users = Array.from({ length: 100 }, (_, i) => ({
  email: `loadtest-user-${i}@example.com`,
  password: "TestPassword123!",
}));

export function setup() {
  // Register test users
  for (const user of users.slice(0, 10)) {
    http.post(`${BASE_URL}/api/v1/auth/register`, JSON.stringify(user), {
      headers: { "Content-Type": "application/json" },
    });
  }
}

export default function () {
  const user = users[Math.floor(Math.random() * users.length)];

  // 1. Login
  const loginStart = Date.now();
  const loginRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email: user.email, password: user.password }),
    { headers: { "Content-Type": "application/json" } }
  );
  loginDuration.add(Date.now() - loginStart);

  const loginOk = check(loginRes, {
    "login status 200": (r) => r.status === 200,
    "login has access_token": (r) => r.json("access_token") !== "",
  });
  errorRate.add(!loginOk);

  if (!loginOk) {
    sleep(1);
    return;
  }

  const token = loginRes.json("access_token");
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };

  // 2. Get user profile
  const profileRes = http.get(`${BASE_URL}/api/v1/users/me`, { headers });
  check(profileRes, { "profile status 200": (r) => r.status === 200 });
  errorRate.add(profileRes.status !== 200);

  // 3. Create order
  const orderStart = Date.now();
  const orderRes = http.post(
    `${BASE_URL}/api/v1/orders`,
    JSON.stringify({
      items: [
        { product_id: "prod-001", name: "Widget A", quantity: 2, unit_price: 9.99 },
        { product_id: "prod-002", name: "Widget B", quantity: 1, unit_price: 24.99 },
      ],
    }),
    { headers }
  );
  orderDuration.add(Date.now() - orderStart);

  const orderOk = check(orderRes, {
    "order status 201": (r) => r.status === 201,
    "order has id": (r) => r.json("id") !== "",
  });
  errorRate.add(!orderOk);

  // 4. List orders
  const ordersRes = http.get(`${BASE_URL}/api/v1/orders`, { headers });
  check(ordersRes, { "orders status 200": (r) => r.status === 200 });

  sleep(Math.random() * 2 + 0.5); // Think time: 0.5–2.5s
}

export function handleSummary(data) {
  return {
    "results/load-test-summary.json": JSON.stringify(data, null, 2),
    stdout: `
========================================
  LOAD TEST SUMMARY
========================================
  Total Requests:  ${data.metrics.http_reqs.values.count}
  Avg Duration:    ${Math.round(data.metrics.http_req_duration.values.avg)}ms
  p95 Duration:    ${Math.round(data.metrics.http_req_duration.values["p(95)"])}ms
  p99.9 Duration:  ${Math.round(data.metrics.http_req_duration.values["p(99.9)"])}ms
  Error Rate:      ${(data.metrics.errors.values.rate * 100).toFixed(4)}%
  
  SLO Status:
  ✓ p99.9 < 200ms: ${data.metrics.http_req_duration.values["p(99.9)"] < 200 ? "PASS" : "FAIL"}
  ✓ Error < 0.1%:  ${data.metrics.errors.values.rate < 0.001 ? "PASS" : "FAIL"}
========================================
`,
  };
}
# load test
// load
// options
// scenarios
// summary
// auth flow
// order flow
// think time
// tenants
// metrics
// options
// scenarios
// summary
// auth flow
// order flow
// think time
// tenants
// metrics
// auth token
// error rate
// rps counter
// batch size
// options
// scenarios
// summary
// auth flow
// order flow
// think time
// tenants
// metrics
// auth token
