// Native portable acceptance. The child runs outside the checkout with an empty PATH.
import { execFileSync, spawn } from "node:child_process";
import { copyFileSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { request } from "node:https";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import assert from "node:assert/strict";

const target = process.platform === "win32" ? "windows" : "linux";
const name = target === "windows" ? "erp.exe" : "erp";
const root = mkdtempSync(join(tmpdir(), "raiz-portable-"));
const executable = join(root, name),
  data = join(root, "data");
const childEnv = { ...process.env };
for (const key of Object.keys(childEnv))
  if (key.toLowerCase() === "path") childEnv[key] = "";
copyFileSync(
  resolve(`artifacts/erp-${target}-amd64${target === "windows" ? ".exe" : ""}`),
  executable,
);
const run = (args) =>
  execFileSync(executable, args, {
    cwd: root,
    env: childEnv,
    encoding: "utf8",
  }).trim();
const probe = createServer();
await new Promise((resolve) => probe.listen(0, "127.0.0.1", resolve));
const port = probe.address().port;
await new Promise((resolve) => probe.close(resolve));
const origin = `https://127.0.0.1:${port}`;
let child,
  exited,
  cookie = "",
  csrf = "",
  ca;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function stop() {
  if (!child) return;
  let timeout;
  child.kill();
  await Promise.race([
    exited,
    new Promise((resolve) => {
      timeout = setTimeout(() => {
        if (child?.exitCode === null) child.kill("SIGKILL");
        resolve();
      }, 5000);
    }),
  ]);
  clearTimeout(timeout);
  await exited;
  child = undefined;
}
async function start() {
  child = spawn(executable, ["serve", "--data-dir", data], {
    cwd: root,
    env: childEnv,
    stdio: ["ignore", "pipe", "pipe"],
  });
  // Drain logs without exposing the one-use installation token.
  child.stdout.resume();
  child.stderr.resume();
  exited = new Promise((resolve) => child.once("exit", resolve));
  for (let attempt = 0; attempt < 100; attempt++) {
    if (child.exitCode !== null)
      throw new Error("El servidor portable terminó antes de estar listo.");
    try {
      await call("/api/v1/health");
      return;
    } catch {
      await sleep(50);
    }
  }
  throw new Error("El servidor portable no inició.");
}
function call(path, method = "GET", body, headers = {}, expected = 200) {
  return new Promise((resolve, reject) => {
    const payload = body === undefined ? undefined : JSON.stringify(body);
    const req = request(
      new URL(path, origin),
      {
        method,
        ca,
        headers: {
          ...(cookie ? { Cookie: cookie } : {}),
          ...(csrf ? { "X-CSRF-Token": csrf } : {}),
          ...(payload ? { "Content-Type": "application/json" } : {}),
          ...headers,
        },
      },
      (response) => {
        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => {
          const raw = Buffer.concat(chunks).toString("utf8");
          if (response.statusCode !== expected)
            return reject(
              new Error(`${method} ${path}: ${response.statusCode} ${raw}`),
            );
          if (response.headers["set-cookie"])
            cookie = response.headers["set-cookie"]
              .map((value) => value.split(";")[0])
              .join("; ");
          if (response.headers["content-type"]?.includes("application/json")) {
            try {
              resolve(JSON.parse(raw));
            } catch (error) {
              reject(error);
            }
          } else resolve(raw);
        });
      },
    );
    req.setTimeout(5000, () =>
      req.destroy(new Error("Tiempo de espera agotado.")),
    );
    req.on("error", reject);
    req.end(payload);
  });
}
const credentials = {
  username: "admin",
  password: "Prueba portable segura 2026",
};
async function login() {
  const session = await call("/api/v1/auth/login", "POST", credentials);
  csrf = session.csrfToken;
}
try {
  run(["init", "--data-dir", data, "--listen", `127.0.0.1:${port}`, "--tls"]);
  ca = readFileSync(join(data, "certificates", "ca.crt"));
  await start();
  const token = readFileSync(join(data, "setup.token"), "utf8");
  await call(
    "/api/v1/setup",
    "POST",
    {
      company: {
        name: "Empresa portable",
        currency: "CLP",
        timezone: "America/Santiago",
      },
      name: "Administrador",
      ...credentials,
    },
    { Authorization: `Setup ${token}` },
    201,
  );
  await login();
  const html = await call("/");
  const asset = html.match(/src="(\/assets\/[^\"]+)"/)?.[1];
  assert.ok(asset, "La interfaz debe estar incorporada en el ejecutable.");
  assert.ok((await call(asset)).length > 100);
  const product = await call(
    "/api/v1/products",
    "POST",
    {
      sku: "PORT-001",
      name: "Producto portable",
      unit: "un",
      priceMinor: "1250",
    },
    {},
    201,
  );
  const receipt = (count) =>
    call(
      "/api/v1/inventory/receipts",
      "POST",
      { productId: product.id, quantity: count, reason: "Prueba portable" },
      {},
      201,
    );
  await receipt("10");
  const saleRequest = { lines: [{ productId: product.id, quantity: "2" }] };
  const quote = await call("/api/v1/sales/quote", "POST", saleRequest);
  const sale = await call(
    "/api/v1/sales",
    "POST",
    { ...saleRequest, quoteFingerprint: quote.fingerprint },
    { "Idempotency-Key": "portable-smoke-sale-0001" },
    201,
  );
  assert.equal(sale.totalMinor, "2500");
  const expenseRequest = {
    amountMinor: "600",
    reason: "Papel para el negocio",
  };
  const expenseHeaders = { "Idempotency-Key": "portable-smoke-expense-0001" };
  const expense = await call(
    "/api/v1/cash/expenses",
    "POST",
    expenseRequest,
    expenseHeaders,
    201,
  );
  assert.equal(expense.amountMinor, "-600");
  assert.equal(expense.balanceAfter, "1900");
  const archive = run(["backup", "--data-dir", data]);
  await receipt("3");
  // Restore the oldest of seven copies: the pre-restore safety backup rotates
  // that original path, so the CLI must stage it before applying retention.
  for (let index = 0; index < 6; index++) run(["backup", "--data-dir", data]);
  await stop();
  run(["restore", "--data-dir", data, "--file", archive]);
  await start();
  await call("/api/v1/auth/me", "GET", undefined, {}, 401);
  await login();
  const products = await call("/api/v1/inventory/stocks");
  assert.equal(products.items[0].stock, "8.000000");
  const summary = await call("/api/v1/reports/summary");
  assert.equal(summary.cashMinor, "1900");
  assert.equal(summary.cashIncomeMinor, "2500");
  assert.equal(summary.cashExpenseMinor, "600");
  const recoveredExpense = await call(
    "/api/v1/cash/expenses",
    "POST",
    expenseRequest,
    expenseHeaders,
  );
  assert.equal(recoveredExpense.id, expense.id);
  assert.equal((await call("/api/v1/cash/movements?kind=expense")).total, 1);
  assert.equal(summary.sales, 1);
  await stop();
  const diagnostic = JSON.parse(run(["doctor", "--data-dir", data]));
  assert.equal(diagnostic.status, "ok");
  assert.equal(diagnostic.foreignKeys, true);
  assert.equal(diagnostic.synchronous, 2);
  // Console recovery explicitly permits a blank credential; no runtime is on PATH.
  execFileSync(
    executable,
    [
      "reset-password",
      "--data-dir",
      data,
      "--username",
      "admin",
      "--allow-weak-password",
    ],
    { cwd: root, env: childEnv, input: "\n", encoding: "utf8" },
  );
  credentials.password = "";
  await start();
  await login();
  await call(
    "/api/v1/auth/change-password",
    "POST",
    { currentPassword: "", newPassword: "1", allowWeakPassword: true },
    {},
    204,
  );
  credentials.password = "1";
  await login();
  await stop();
  console.log(
    `Portable ${target}: HTTPS, recursos incorporados, venta, egreso, contraseñas opcionales, respaldo, restauración y reinicio correctos (PATH vacío).`,
  );
} finally {
  await stop();
  rmSync(root, { recursive: true, force: true });
}
