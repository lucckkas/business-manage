// Canonical API contract. Generate JSON with `node api/openapi.mjs`.
import { writeFileSync } from "node:fs";
import { extendSchemas, extendPaths } from "./commercial.mjs";
import { peripheralSchemas, peripheralPaths } from "./peripherals.mjs";
const str = (extra = {}) => ({ type: "string", ...extra });
const integer = (extra = {}) => ({ type: "integer", ...extra });
const boolean = { type: "boolean" };
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const obj = (properties, required = Object.keys(properties)) => ({
  type: "object",
  additionalProperties: false,
  properties,
  required,
});
const array = (items) => ({ type: "array", items });
const id = str({ pattern: "^[a-f0-9]{32}$" });
const date = str({ format: "date-time" });
const minor = str({
  pattern: "^[0-9]+$",
  description:
    "Importe entero exacto en unidades monetarias menores; nunca un número JSON.",
});
const quantity = str({
  pattern: "^[0-9]+(\\.[0-9]{1,3})?$",
  description: "Cantidad decimal exacta con hasta tres decimales.",
});
const role = str({ enum: ["admin", "operator"] });
const companyInput = {
  name: str({ minLength: 2, maxLength: 160 }),
  currency: str({
    enum: [
      "CLP",
      "USD",
      "EUR",
      "PEN",
      "MXN",
      "ARS",
      "COP",
      "UYU",
      "BOB",
      "BRL",
    ],
  }),
  timezone: str({ example: "America/Santiago" }),
};
const productFields = {
  sku: str({ minLength: 1, maxLength: 64 }),
  name: str({ minLength: 2, maxLength: 160 }),
  unit: str({ enum: ["un", "kg", "lt", "mt"] }),
  priceMinor: minor,
};
const userInput = {
  username: str({ minLength: 3, maxLength: 64 }),
  name: str({ minLength: 2, maxLength: 120 }),
  password: str({
    maxLength: 128,
    format: "password",
    description:
      "Menos de 12 caracteres, incluido vacío, requiere allowWeakPassword.",
  }),
  role,
  allowWeakPassword: boolean,
};
const product = { id, ...productFields, createdAt: date };
const saleFields = {
  id,
  number: integer(),
  totalMinor: minor,
  actorId: id,
  createdAt: date,
  lines: array(ref("SaleLine")),
};
const schemas = {
  Error: obj({ error: obj({ code: str(), message: str(), requestId: str() }) }),
  Company: obj({
    ...companyInput,
    currencyScale: integer({ minimum: 0, maximum: 3 }),
  }),
  SetupStatus: obj({ initialized: boolean, localSetupAllowed: boolean }),
  SetupRequest: obj(
    {
      company: obj(companyInput),
      username: userInput.username,
      name: userInput.name,
      password: userInput.password,
      allowWeakPassword: boolean,
    },
    ["company", "username", "name", "password"],
  ),
  SetupResult: obj({ initialized: boolean }),
  User: obj({
    id,
    username: str(),
    name: str(),
    role,
    active: boolean,
    createdAt: date,
  }),
  CreateUser: obj(userInput, ["username", "name", "password", "role"]),
  SetUserActive: obj({ active: boolean }),
  Login: obj({
    username: str(),
    password: str({ maxLength: 128, format: "password" }),
  }),
  Session: obj({
    user: ref("User"),
    company: ref("Company"),
    csrfToken: str(),
  }),
  CreateProduct: obj(productFields),
  Product: obj(product),
  ProductView: obj({ ...product, stock: quantity }),
  ReceiveStock: obj({
    productId: id,
    quantity,
    reason: str({ minLength: 3, maxLength: 240 }),
  }),
  Receipt: obj({ id, productId: id, quantity, reason: str(), createdAt: date }),
  SaleRequest: obj({
    lines: {
      ...array(obj({ productId: id, quantity })),
      minItems: 1,
      maxItems: 100,
    },
  }),
  SaleLine: obj({
    productId: id,
    sku: str(),
    name: str(),
    unit: str(),
    quantity,
    priceMinor: minor,
    totalMinor: minor,
  }),
  Sale: obj(saleFields),
  CashExpense: obj({
    amountMinor: { ...minor, description: "Monto mayor que cero." },
    reason: str({ minLength: 3, maxLength: 240 }),
  }),
  CashMovement: obj(
    {
      id,
      kind: str({ enum: ["sale", "expense"] }),
      reference: str(),
      saleId: id,
      saleNumber: integer(),
      expenseNumber: integer(),
      amountMinor: str({ pattern: "^-?[0-9]+$" }),
      actorId: id,
      actorName: str(),
      reason: str(),
      createdAt: date,
      balanceBefore: str({ pattern: "^-?[0-9]+$" }),
      balanceAfter: str({ pattern: "^-?[0-9]+$" }),
      drawerJobId: id,
    },
    [
      "id",
      "kind",
      "reference",
      "amountMinor",
      "actorId",
      "actorName",
      "reason",
      "createdAt",
    ],
  ),
  AuditEntry: obj({
    id,
    actorId: str(),
    action: str(),
    entityId: str(),
    createdAt: date,
  }),
  Summary: obj({
    products: integer(),
    sales: integer(),
    revenueMinor: minor,
    cashMinor: str({ pattern: "^-?[0-9]+$" }),
    cashIncomeMinor: minor,
    cashExpenseMinor: minor,
    outOfStock: integer(),
    todaySales: integer(),
    todayRevenueMinor: minor,
  }),
  Backup: obj({ filename: str() }),
  Health: obj({ status: str(), version: str() }),
  Diagnostics: obj({
    version: str(),
    uptimeSeconds: integer(),
    requests: integer(),
    heapBytes: integer(),
    runtimeBytes: integer(),
  }),
};
for (const [name, item] of Object.entries({
  ProductPage: "ProductView",
  SalePage: "Sale",
  CashPage: "CashMovement",
  UserPage: "User",
  AuditPage: "AuditEntry",
})) {
  schemas[name] = obj({
    items: array(ref(item)),
    total: integer(),
    page: integer(),
    pageSize: integer(),
  });
}
extendSchemas(schemas, {
  obj,
  str,
  ref,
  array,
  id,
  date,
  minor,
  quantity,
  boolean,
  integer,
});
peripheralSchemas(schemas, {
  obj,
  str,
  ref,
  array,
  id,
  date,
  minor,
  quantity,
  boolean,
  integer,
});
const paths = {};
const json = (schema) => ({ "application/json": { schema } });
const errors = Object.fromEntries(
  [400, 401, 403, 404, 409, 429, 500, 503].map((status) => [
    status,
    {
      description: "Error uniforme; el código identifica la causa.",
      content: json(ref("Error")),
    },
  ]),
);
function endpoint(path, method, operationId, summary, response, options = {}) {
  const parameters = [];
  if (options.paged)
    parameters.push(
      ...[
        { name: "search", in: "query", schema: str({ maxLength: 160 }) },
        {
          name: "page",
          in: "query",
          schema: integer({ minimum: 1, maximum: 1000000, default: 1 }),
        },
        {
          name: "pageSize",
          in: "query",
          schema: integer({ minimum: 1, maximum: 100, default: 20 }),
        },
      ],
    );
  if (path.includes("{id}"))
    parameters.push({ name: "id", in: "path", required: true, schema: id });
  if (!options.public && !["get", "head"].includes(method))
    parameters.push({
      name: "X-CSRF-Token",
      in: "header",
      required: true,
      schema: str(),
    });
  if (options.idempotent)
    parameters.push({
      name: "Idempotency-Key",
      in: "header",
      required: true,
      schema: str({ minLength: 16, maxLength: 128 }),
      description:
        "Reutilizar la misma clave y cuerpo ante reintentos. Otro cuerpo con la misma clave produce 409.",
    });
  if (operationId === "setup")
    parameters.push({
      name: "Authorization",
      in: "header",
      required: true,
      schema: str(),
      description: "Setup <token de uso único>. Solo conexiones loopback.",
    });
  const status = options.status ?? (method === "post" ? 201 : 200);
  const success = response
    ? { description: "Operación completada.", content: json(ref(response)) }
    : { description: "Operación completada sin contenido." };
  const responses = { ...errors, [status]: success };
  if (options.idempotent)
    responses[200] = {
      ...success,
      description: "Operación previamente confirmada; respuesta del reintento.",
      headers: { "Idempotency-Replayed": { schema: str({ enum: ["true"] }) } },
    };
  paths[path] ??= {};
  paths[path][method] = {
    operationId,
    summary,
    tags: [options.tag ?? path.split("/")[3]],
    security: options.public ? [] : [{ session: [] }],
    parameters,
    responses,
    ...(options.body
      ? { requestBody: { required: true, content: json(ref(options.body)) } }
      : {}),
  };
}
endpoint(
  "/api/v1/health",
  "get",
  "health",
  "Estado del proceso y conexión de datos",
  "Health",
  {
    public: true,
  },
);
endpoint(
  "/api/v1/setup",
  "get",
  "setupStatus",
  "Consultar si la empresa está configurada",
  "SetupStatus",
  { public: true },
);
endpoint(
  "/api/v1/setup",
  "post",
  "setup",
  "Configurar empresa y administrador una sola vez",
  "SetupResult",
  { public: true, body: "SetupRequest" },
);
endpoint(
  "/api/v1/auth/login",
  "post",
  "login",
  "Iniciar sesión por 12 horas",
  "Session",
  {
    public: true,
    body: "Login",
    status: 200,
  },
);
endpoint(
  "/api/v1/auth/me",
  "get",
  "me",
  "Sesión, empresa y token CSRF actuales",
  "Session",
);
endpoint(
  "/api/v1/auth/logout",
  "post",
  "logout",
  "Cerrar la sesión actual",
  null,
  { status: 204 },
);
endpoint(
  "/api/v1/products",
  "get",
  "products",
  "Buscar catálogo y existencias",
  "ProductPage",
  {
    paged: true,
  },
);
endpoint(
  "/api/v1/products",
  "post",
  "createProduct",
  "Crear producto (administrador)",
  "Product",
  {
    body: "CreateProduct",
  },
);
endpoint(
  "/api/v1/inventory/stocks",
  "get",
  "stocks",
  "Consultar existencias de la bodega principal",
  "ProductPage",
  { paged: true },
);
endpoint(
  "/api/v1/inventory/receipts",
  "post",
  "receiveStock",
  "Ingresar existencias con motivo (administrador)",
  "Receipt",
  { body: "ReceiveStock" },
);
endpoint(
  "/api/v1/sales",
  "get",
  "sales",
  "Listar ventas; el detalle se obtiene por id",
  "SalePage",
  { paged: true },
);
endpoint(
  "/api/v1/sales/{id}",
  "get",
  "sale",
  "Consultar comprobante interno",
  "Sale",
);
endpoint(
  "/api/v1/sales",
  "post",
  "confirmSale",
  "Confirmar venta al contado de forma atómica",
  "Sale",
  { body: "SaleRequest", idempotent: true },
);
endpoint(
  "/api/v1/cash/movements",
  "get",
  "cash",
  "Consultar ingresos y egresos de caja",
  "CashPage",
  {
    paged: true,
  },
);
paths["/api/v1/cash/movements"].get.parameters.push({
  name: "kind",
  in: "query",
  schema: str({ enum: ["sale", "expense"] }),
});
endpoint(
  "/api/v1/cash/movements/{id}",
  "get",
  "cashMovement",
  "Consultar detalle de movimiento de caja",
  "CashMovement",
);
endpoint(
  "/api/v1/cash/expenses",
  "post",
  "cashExpense",
  "Registrar egreso de caja con permiso e idempotencia",
  "CashMovement",
  { body: "CashExpense", idempotent: true },
);
endpoint(
  "/api/v1/reports/summary",
  "get",
  "summary",
  "Resumen acumulado y del día en la zona horaria de la empresa",
  "Summary",
);
endpoint(
  "/api/v1/users",
  "get",
  "users",
  "Listar usuarios (administrador)",
  "UserPage",
  {
    paged: true,
  },
);
endpoint(
  "/api/v1/users",
  "post",
  "createUser",
  "Crear usuario (administrador)",
  "User",
  {
    body: "CreateUser",
  },
);
endpoint(
  "/api/v1/users/{id}",
  "patch",
  "setUserActive",
  "Activar o desactivar una cuenta; revoca sesiones al desactivar",
  null,
  { body: "SetUserActive", status: 204 },
);
endpoint(
  "/api/v1/audit",
  "get",
  "audit",
  "Consultar auditoría (administrador)",
  "AuditPage",
  {
    paged: true,
  },
);
endpoint(
  "/api/v1/system/backups",
  "post",
  "backup",
  "Crear respaldo consistente en el servidor (administrador)",
  "Backup",
);
endpoint(
  "/api/v1/system/diagnostics",
  "get",
  "diagnostics",
  "Diagnóstico del servidor (administrador)",
  "Diagnostics",
);
endpoint(
  "/api/v1/openapi.json",
  "get",
  "openapi",
  "Descargar este contrato (administrador)",
  null,
);
paths["/api/v1/openapi.json"].get.responses[200] = {
  description: "Documento OpenAPI 3.0.3.",
  content: json({ type: "object" }),
};
extendPaths(endpoint, paths);
peripheralPaths(endpoint, paths);
const spec = {
  openapi: "3.0.3",
  info: {
    title: "Raíz ERP · API local",
    version: "0.4.0",
    description:
      "Una empresa por instalación. Operaciones internas sin validez tributaria. Importes y cantidades exactos como cadenas. Sesión mediante cookie HttpOnly y CSRF en escrituras.",
  },
  servers: [{ url: "/" }],
  paths,
  components: {
    securitySchemes: {
      session: { type: "apiKey", in: "cookie", name: "erp_session" },
    },
    schemas,
  },
};
writeFileSync(
  new URL("openapi.json", import.meta.url),
  `${JSON.stringify(spec, null, 2)}\n`,
);
