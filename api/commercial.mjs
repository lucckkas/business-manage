// Schema-2 commercial additions to the canonical OpenAPI generator.
export function extendSchemas(s, h) {
  const { obj, str, ref, array, id, date, minor, quantity, boolean, integer } =
    h;
  const stock = str({
    pattern: "^-?[0-9]+(\\.[0-9]{1,6})?$",
    description: "Saldo exacto con hasta seis decimales.",
  });
  const component = obj({ productId: id, quantity });
  const features = {
    sellable: boolean,
    stockMode: str({ enum: ["own", "components"] }),
    stockPolicy: str({ enum: ["block", "negative", "zero", "untracked"] }),
    components: array(component),
    version: integer(),
  };
  s.Product.properties = { ...s.Product.properties, ...features };
  s.Product.required = Object.keys(s.Product.properties);
  s.ProductView = obj({
    ...s.Product.properties,
    stock: { ...stock, nullable: true },
    availableQuantity: { ...quantity, nullable: true },
  });
  s.CreateProduct.properties = { ...s.CreateProduct.properties, ...features };
  s.CreateProduct.required = ["sku", "name", "unit", "priceMinor"];
  s.UpdateProduct = obj(
    {
      ...s.CreateProduct.properties,
      countedStock: stock,
      adjustmentReason: str(),
    },
    [
      "sku",
      "name",
      "unit",
      "priceMinor",
      "sellable",
      "stockMode",
      "stockPolicy",
      "components",
      "version",
    ],
  );
  s.Discount = obj(
    {
      type: str({ enum: ["percentage", "amount"] }),
      value: str({
        description:
          "Porcentaje hasta dos decimales o importe entero en unidades menores.",
      }),
      basis: str({ enum: ["line", "unit"] }),
    },
    ["type", "value"],
  );
  s.LineDiscount = obj({ productId: id, discount: ref("Discount") });
  s.RewardSelection = obj({ promotionId: id, productId: id, quantity });
  s.SaleRequest.properties = {
    ...s.SaleRequest.properties,
    lineDiscounts: array(ref("LineDiscount")),
    totalDiscount: ref("Discount"),
    discountReason: str({ maxLength: 240 }),
    rewards: array(ref("RewardSelection")),
    quoteFingerprint: str(),
  };
  const promotion = {
    id,
    name: str(),
    kind: str({ enum: ["discount", "buy_pay", "buy_get"] }),
    scope: str({ enum: ["products", "cart"] }),
    productIds: array(id),
    rewardProductIds: array(id),
    discount: ref("Discount"),
    buyQuantity: quantity,
    payQuantity: quantity,
    giftQuantity: quantity,
    startsAt: { ...date, nullable: true },
    endsAt: { ...date, nullable: true },
    enabled: boolean,
    stackable: boolean,
    priority: integer({ minimum: 0, maximum: 100000 }),
    version: integer(),
  };
  s.Promotion = obj(promotion, [
    "id",
    "name",
    "kind",
    "scope",
    "productIds",
    "rewardProductIds",
    "startsAt",
    "endsAt",
    "enabled",
    "stackable",
    "priority",
    "version",
  ]);
  s.SavePromotion = obj(promotion, [
    "name",
    "kind",
    "scope",
    "productIds",
    "rewardProductIds",
    "startsAt",
    "endsAt",
    "enabled",
    "stackable",
    "priority",
  ]);
  s.PromotionPage = obj({
    items: array(ref("Promotion")),
    total: integer(),
    page: integer(),
    pageSize: integer(),
  });
  s.Consumption = obj({
    productId: id,
    sku: str(),
    name: str(),
    unit: str(),
    quantity: stock,
    factor: quantity,
    stockPolicy: features.stockPolicy,
  });
  s.AppliedDiscount = obj(
    {
      kind: str(),
      lineId: str(),
      label: str(),
      amountMinor: minor,
      specification: ref("Discount"),
      promotion: ref("Promotion"),
      actorId: id,
      actorName: str(),
      reason: str(),
    },
    ["kind", "label", "amountMinor"],
  );
  s.SaleLine.properties = {
    ...s.SaleLine.properties,
    id: str(),
    kind: str({ enum: ["regular", "gift"] }),
    promotionId: str(),
    grossMinor: minor,
    promotionDiscountMinor: minor,
    manualDiscountMinor: minor,
    orderDiscountMinor: minor,
    consumptions: array(ref("Consumption")),
  };
  s.SaleLine.required = Object.keys(s.SaleLine.properties).filter(
    (k) => k !== "promotionId",
  );
  const totals = {
    lines: array(ref("SaleLine")),
    subtotalMinor: minor,
    promotionDiscountMinor: minor,
    manualDiscountMinor: minor,
    totalMinor: minor,
    discounts: array(ref("AppliedDiscount")),
  };
  s.Sale.properties = {
    ...s.Sale.properties,
    ...totals,
    company: ref("Company"),
  };
  s.Sale.required = Object.keys(s.Sale.properties);
  s.RewardOption = obj({
    promotionId: id,
    name: str(),
    maximumQuantity: quantity,
    selectedQuantity: quantity,
    products: array(
      obj({ productId: id, name: str(), sku: str(), unit: str() }),
    ),
  });
  s.Availability = obj({
    productId: id,
    name: str(),
    requiredQuantity: stock,
    availableQuantity: { ...stock, nullable: true },
    sufficient: boolean,
  });
  s.SaleQuote = obj({
    ...totals,
    fingerprint: str(),
    rewardOptions: array(ref("RewardOption")),
    availability: array(ref("Availability")),
  });
  s.ReceiveStock.properties.quantity = stock;
  s.Receipt.properties.quantity = stock;
  s.StockAdjustment = obj({
    id,
    productId: id,
    countedQuantity: stock,
    reason: str({ minLength: 3, maxLength: 240 }),
    createdAt: date,
  });
  s.AdjustStock = obj({
    productId: id,
    countedQuantity: stock,
    reason: str({ minLength: 3, maxLength: 240 }),
  });
  s.User.properties.canApplyManualDiscounts = boolean;
  s.User.properties.canRecordCashExpenses = boolean;
  s.User.required.push("canRecordCashExpenses");
  s.CreateUser.properties.canRecordCashExpenses = boolean;
  s.User.required.push("canApplyManualDiscounts");
  s.CreateUser.properties.canApplyManualDiscounts = boolean;
  s.SetUserActive = obj(
    {
      active: boolean,
      canApplyManualDiscounts: boolean,
      canRecordCashExpenses: boolean,
    },
    [],
  );
  s.SetUserActive.minProperties = 1;
  s.SetUserActive.maxProperties = 1;
  s.UpdateProfile = obj({ name: str({ minLength: 2, maxLength: 120 }) });
  s.ChangePassword = obj(
    {
      currentPassword: str({ format: "password", maxLength: 128 }),
      newPassword: str({ format: "password", maxLength: 128 }),
      allowWeakPassword: boolean,
    },
    ["currentPassword", "newPassword"],
  );
  s.UpdateCompany = obj({
    name: str({ minLength: 2, maxLength: 160 }),
    timezone: str(),
  });
  s.AuditEntry.properties.details = {
    type: "object",
    additionalProperties: true,
  };
}
export function extendPaths(endpoint, paths) {
  endpoint(
    "/api/v1/products/{id}",
    "get",
    "product",
    "Consultar producto y composición",
    "Product",
  );
  endpoint(
    "/api/v1/products/{id}",
    "patch",
    "updateProduct",
    "Editar producto sin alterar su historial (administrador)",
    "Product",
    { body: "UpdateProduct" },
  );
  endpoint(
    "/api/v1/inventory/adjustments",
    "post",
    "adjustStock",
    "Registrar existencias contadas y movimiento de ajuste",
    "StockAdjustment",
    { body: "AdjustStock" },
  );
  endpoint(
    "/api/v1/promotions",
    "get",
    "promotions",
    "Listar ofertas y descuentos (administrador)",
    "PromotionPage",
    { paged: true },
  );
  endpoint(
    "/api/v1/promotions",
    "post",
    "createPromotion",
    "Crear una regla comercial (administrador)",
    "Promotion",
    { body: "SavePromotion" },
  );
  endpoint(
    "/api/v1/promotions/{id}",
    "get",
    "promotion",
    "Consultar una regla comercial (administrador)",
    "Promotion",
  );
  endpoint(
    "/api/v1/promotions/{id}",
    "patch",
    "updatePromotion",
    "Editar o pausar una regla por versión (administrador)",
    "Promotion",
    { body: "SavePromotion" },
  );
  endpoint(
    "/api/v1/sales/quote",
    "post",
    "quoteSale",
    "Cotizar ofertas, descuentos, regalos e insumos sin registrar la venta",
    "SaleQuote",
    { body: "SaleRequest", status: 200 },
  );
  endpoint(
    "/api/v1/auth/me",
    "patch",
    "updateProfile",
    "Cambiar el nombre visible de la propia cuenta",
    "User",
    { body: "UpdateProfile" },
  );
  endpoint(
    "/api/v1/auth/change-password",
    "post",
    "changePassword",
    "Cambiar contraseña y revocar todas las sesiones",
    null,
    { body: "ChangePassword", status: 204 },
  );
  endpoint(
    "/api/v1/company",
    "patch",
    "updateCompany",
    "Modificar nombre y zona horaria sin cambiar moneda",
    "Company",
    { body: "UpdateCompany" },
  );
  for (const path of ["/api/v1/products", "/api/v1/inventory/stocks"])
    paths[path].get.parameters.push(
      { name: "sellable", in: "query", schema: { type: "boolean" } },
      {
        name: "stockMode",
        in: "query",
        schema: { type: "string", enum: ["own"] },
      },
    );
}
