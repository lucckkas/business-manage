// Schema 3: audit projections, barcode identities, persistent peripheral jobs.
export function peripheralSchemas(s, h) {
  const { obj, str, ref, array, id, date, boolean, integer } = h;
  s.Barcode = obj(
    {
      value: str({ minLength: 1, maxLength: 64 }),
      format: str({ enum: ["CODE128", "EAN8", "EAN13", "UPC"] }),
      generated: boolean,
      primary: boolean,
    },
    ["value", "format"],
  );
  for (const name of [
    "Product",
    "ProductView",
    "CreateProduct",
    "UpdateProduct",
  ]) {
    s[name].properties.barcodes = array(ref("Barcode"));
    if (["Product", "ProductView"].includes(name))
      s[name].required.push("barcodes");
  }
  const auditFields = {
    actorName: str(),
    label: str(),
    reference: str(),
    summary: str(),
    subjectType: str(),
    subjectId: str(),
    historical: boolean,
  };
  Object.assign(s.AuditEntry.properties, auditFields);
  s.AuditEntry.required.push(...Object.keys(auditFields));
  s.AuditOperation = obj({ code: str(), label: str(), subjectType: str() });
  s.AuditOperations = array(ref("AuditOperation"));
  s.AuditDetail = obj({ entry: ref("AuditEntry"), sale: ref("Sale") }, [
    "entry",
  ]);
  s.POSDevice = obj({ id: str(), name: str(), transport: str() });
  s.POSDevices = array(ref("POSDevice"));
  s.POSSettings = obj({
    version: integer(),
    deviceId: str(),
    deviceName: str(),
    width: integer({ enum: [58, 80] }),
    cutter: boolean,
    drawerPin: integer({ enum: [0, 1] }),
    pulseOnMs: integer({ minimum: 2, maximum: 510 }),
    pulseOffMs: integer({ minimum: 2, maximum: 510 }),
    enabled: boolean,
    drawerEnabled: boolean,
    printVerified: boolean,
    drawerVerified: boolean,
  });
  s.PrintJob = obj(
    {
      id,
      saleId: id,
      saleNumber: integer(),
      cashMovementId: id,
      kind: str({
        enum: [
          "sale",
          "copy",
          "test_print",
          "test_drawer",
          "drawer",
          "expense_drawer",
        ],
      }),
      state: str({
        enum: ["pending", "sending", "sent", "error", "uncertain", "suspended"],
      }),
      actorId: id,
      createdAt: date,
      updatedAt: date,
      opensDrawer: boolean,
      reason: str(),
      error: str(),
      spoolId: str(),
      printerName: str(),
    },
    [
      "id",
      "kind",
      "state",
      "actorId",
      "createdAt",
      "updatedAt",
      "opensDrawer",
      "reason",
      "error",
      "spoolId",
      "printerName",
    ],
  );
  s.PrintJobPage = obj({
    items: array(ref("PrintJob")),
    total: integer(),
    page: integer(),
    pageSize: integer(),
  });
  s.PrintRequest = obj(
    {
      kind: str({ enum: ["test_print", "test_drawer", "copy", "drawer"] }),
      reason: str({ maxLength: 240 }),
    },
    [],
  );
  s.VerifyPOSTest = obj({ jobId: id });
  s.Sale.properties.printJobId = id;
}
export function peripheralPaths(endpoint, paths) {
  endpoint(
    "/api/v1/products/lookup",
    "get",
    "lookupBarcode",
    "Buscar código exacto conservando ceros iniciales",
    "ProductView",
  );
  paths["/api/v1/products/lookup"].get.parameters.push({
    name: "barcode",
    in: "query",
    required: true,
    schema: { type: "string", minLength: 1, maxLength: 64 },
  });
  endpoint(
    "/api/v1/products/{id}/barcodes/generate",
    "post",
    "generateBarcode",
    "Obtener o generar el código interno estable (administrador)",
    "ProductView",
    { status: 200 },
  );
  endpoint(
    "/api/v1/audit/operations",
    "get",
    "auditOperations",
    "Catálogo de operaciones en español (administrador)",
    "AuditOperations",
  );
  endpoint(
    "/api/v1/audit/{id}",
    "get",
    "auditDetail",
    "Detalle histórico y referencias de una operación (administrador)",
    "AuditDetail",
  );
  paths["/api/v1/audit"].get.parameters.push({
    name: "action",
    in: "query",
    schema: { type: "string" },
    description:
      "Código exacto elegido del catálogo de operaciones. search admite nombres parciales en español.",
  });
  endpoint(
    "/api/v1/pos/settings",
    "get",
    "posSettings",
    "Consultar configuración de impresión",
    "POSSettings",
  );
  endpoint(
    "/api/v1/pos/settings",
    "put",
    "savePOSSettings",
    "Guardar perfil; verificación física requerida para activar (administrador)",
    "POSSettings",
    { body: "POSSettings" },
  );
  endpoint(
    "/api/v1/pos/devices",
    "get",
    "posDevices",
    "Impresoras del servidor (administrador)",
    "POSDevices",
  );
  endpoint(
    "/api/v1/pos/jobs",
    "get",
    "posJobs",
    "Consultar trabajos; enviado no confirma salida de papel",
    "PrintJobPage",
    { paged: true },
  );
  paths["/api/v1/pos/jobs"].get.parameters.push({
    name: "saleId",
    in: "query",
    schema: { type: "string" },
  });
  endpoint(
    "/api/v1/pos/jobs/{id}",
    "get",
    "posJob",
    "Consultar estado de un trabajo",
    "PrintJob",
  );
  endpoint(
    "/api/v1/pos/tests",
    "post",
    "posTest",
    "Solicitar prueba de ticket o cajón (administrador)",
    "PrintJob",
    { body: "PrintRequest", status: 202, idempotent: true },
  );
  endpoint(
    "/api/v1/pos/verify",
    "post",
    "posVerify",
    "Confirmar resultado físico de una prueba enviada (administrador)",
    "POSSettings",
    { body: "VerifyPOSTest", status: 200 },
  );
  endpoint(
    "/api/v1/pos/drawer/open",
    "post",
    "openDrawer",
    "Abrir cajón con motivo (administrador)",
    "PrintJob",
    { body: "PrintRequest", status: 202, idempotent: true },
  );
  endpoint(
    "/api/v1/sales/{id}/print",
    "post",
    "printSale",
    "Solicitar copia sin apertura de cajón",
    "PrintJob",
    { body: "PrintRequest", status: 202, idempotent: true },
  );
}
