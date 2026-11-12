import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { unzipSync, strFromU8 } from 'fflate';

test.describe.configure({ mode: 'serial' });

async function loginCurrent(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.getByLabel('Usuario', { exact: true }).fill('maria');
  await page.getByLabel('Contraseña', { exact: true }).fill('Mi nueva frase segura 2026');
  await page.getByRole('button', { name: 'Iniciar sesión', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Productos', exact: true })).toBeVisible();
}

async function expectTotalDiscountActions(page: import('@playwright/test').Page) {
  const group = page.getByRole('group', { name: 'Descuento sobre el total', exact: true });
  await group.scrollIntoViewIfNeeded();
  const edit = await group
    .getByRole('button', { name: 'Editar descuento total', exact: true })
    .boundingBox();
  const remove = await group
    .getByRole('button', { name: 'Quitar descuento total', exact: true })
    .boundingBox();
  const bounds = await group.boundingBox();
  const payment = await page.locator('.payment-method').boundingBox();
  expect(edit && remove && bounds && payment).toBeTruthy();
  expect(remove!.x - (edit!.x + edit!.width)).toBeGreaterThanOrEqual(10);
  expect(Math.abs(edit!.y + edit!.height / 2 - remove!.y - remove!.height / 2)).toBeLessThanOrEqual(
    1
  );
  expect(remove!.x + remove!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width + 1);
  expect(bounds!.y - payment!.y - payment!.height).toBeGreaterThanOrEqual(10);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
    page.viewportSize()!.width
  );
}

test('alta, catálogo, inventario, venta, comprobante y operación sin Internet', async ({
  page,
  context,
}) => {
  const unexpected: string[] = [];
  page.on('pageerror', (error) => unexpected.push(error.message));
  // Deny every external origin while preserving access to the local server.
  const base = process.env.ERP_BASE_URL ?? 'http://127.0.0.1:8080';
  await context.route('**/*', (route) =>
    new URL(route.request().url()).origin === new URL(base).origin
      ? route.continue()
      : route.abort()
  );
  const token = readFileSync(
    process.env.ERP_TEST_DATA_DIR
      ? `${process.env.ERP_TEST_DATA_DIR}/setup.token`
      : '../data/setup.token',
    'utf8'
  ).trim();
  await page.goto(`/#setup-token=${token}`);
  await page.getByLabel('Nombre de la empresa').fill('Almacén del Sur');
  await page.getByLabel('Tu nombre', { exact: true }).fill('María Torres');
  await page.getByLabel('Usuario', { exact: true }).fill('maria');
  await page.getByLabel('Contraseña', { exact: true }).fill('Una frase segura 2026');
  await expect(page.getByLabel('Código de instalación')).toHaveValue(token);
  await page.getByRole('button', { name: 'Crear mi espacio' }).click();
  await expect(page.getByRole('button', { name: 'Iniciar sesión' })).toBeVisible();
  await page.getByLabel('Usuario', { exact: true }).fill('maria');
  await page.getByLabel('Contraseña', { exact: true }).fill('Una frase segura 2026');
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page.getByRole('heading', { name: 'Tu negocio, en orden.' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Productos', exact: true })).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Inventario', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Agregar mi primer producto' }).click();
  await page.getByLabel('Nombre del producto').fill('Café de especialidad');
  await page.getByLabel('Código SKU').fill('CAF-001');
  await page.getByLabel('Precio de venta (CLP)').fill('4500');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page
    .locator('.quick-panel')
    .getByRole('button', { name: /Ingresar existencias/ })
    .click();
  await expect(page.getByRole('heading', { name: 'Productos', exact: true })).toBeVisible();
  await expect(
    page.getByText('Administra tu catálogo y sus existencias', { exact: true })
  ).toBeVisible();
  for (const name of ['Producto', 'Código', 'Unidad', 'Precio de venta', 'Existencias']) {
    await expect(page.getByRole('columnheader', { name, exact: true })).toBeVisible();
  }
  await page.getByRole('button', { name: 'Ingresar existencias de Café de especialidad' }).click();
  await page.getByLabel('Cantidad a ingresar (un)').fill('10');
  await page.getByLabel('Motivo del ingreso').fill('Existencias iniciales');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText('10 un', { exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/products-unified-desktop.png', fullPage: true });
  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Ventas', exact: true })).toBeVisible();
  const previousHistoryLength = await page.evaluate(() => history.length);
  await page.goto('/#/inventory');
  await expect(page).toHaveURL(/#\/products$/);
  await expect(page.getByRole('heading', { name: 'Productos', exact: true })).toBeVisible();
  expect(await page.evaluate(() => history.length)).toBe(previousHistoryLength + 1);
  await page.goBack();
  await expect(page.getByRole('heading', { name: 'Ventas', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Nueva venta', exact: true }).click();
  await page.getByRole('button', { name: 'Agregar Café de especialidad' }).click();
  await page.getByLabel('Cantidad de Café de especialidad', { exact: true }).fill('2');
  await page.getByRole('button', { name: 'Confirmar venta', exact: true }).click();
  const receipt = page.getByRole('dialog');
  await expect(receipt.getByText('Venta registrada correctamente')).toBeVisible();
  await expect(receipt.getByText('$ 9.000', { exact: true }).first()).toBeVisible();
  await expect(receipt.getByText('Café de especialidad', { exact: true })).toBeVisible();
  await receipt.getByRole('button', { name: 'Cerrar', exact: true }).last().click();
  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await expect(page.getByText('8 un', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Caja', exact: true }).click();
  await expect(page.getByText('+ $ 9.000', { exact: true })).toBeVisible();
  // The server commits a second sale but the response is lost. Reload and retry
  // must recover that same sale rather than deduct stock and collect cash twice.
  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await page.getByRole('button', { name: 'Nueva venta', exact: true }).click();
  await page.getByRole('button', { name: 'Agregar Café de especialidad' }).click();
  await page.route('**/api/v1/sales', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    await route.fetch();
    await route.abort();
  });
  await page.getByRole('button', { name: 'Confirmar venta', exact: true }).click();
  await expect(page.getByText(/La venta está pendiente de confirmación/)).toBeVisible();
  await page.unroute('**/api/v1/sales');
  await page.reload();
  await page.getByRole('button', { name: 'Reintentar confirmación', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('Venta registrada correctamente')).toBeVisible();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();
  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await expect(page.getByText('7 un', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Caja', exact: true }).click();
  await expect(
    page
      .locator('.cash-banner')
      .filter({ hasText: 'Saldo registrado' })
      .getByText('$ 13.500', { exact: true })
  ).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Resumen', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Tu negocio, en orden.' })).toBeVisible();
  await page.screenshot({ path: 'test-results/dashboard-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('heading', { name: 'Tu negocio, en orden.' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: 'test-results/dashboard-mobile.png', fullPage: true });
  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Ingresar existencias de Café de especialidad' })
  ).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: 'test-results/products-unified-mobile.png', fullPage: true });
  expect(unexpected).toEqual([]);
});

test('insumos, ofertas, descuentos, edición, ajustes y carritos por usuario', async ({
  page,
  context,
  browser,
}) => {
  test.setTimeout(90_000);
  const base = process.env.ERP_BASE_URL ?? 'http://127.0.0.1:8080';
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await context.route('**/*', (route) =>
    new URL(route.request().url()).origin === new URL(base).origin
      ? route.continue()
      : route.abort()
  );
  async function login(username = 'maria', password = 'Una frase segura 2026') {
    await page.getByLabel('Usuario', { exact: true }).fill(username);
    await page.getByLabel('Contraseña', { exact: true }).fill(password);
    await page.getByRole('button', { name: 'Iniciar sesión', exact: true }).click();
    await expect(page.getByRole('link', { name: 'Ajustes', exact: true })).toBeVisible();
  }
  await page.goto('/');
  await login();
  const session = await (await page.request.get('/api/v1/auth/me')).json();
  const csrf = { 'X-CSRF-Token': session.csrfToken };
  async function post(path: string, data: unknown) {
    const response = await page.request.post(`/api/v1${path}`, { headers: csrf, data });
    expect(response.status(), await response.text()).toBe(201);
    return response.json();
  }
  const paper = await post('/products', {
    name: 'Hoja A4',
    sku: 'A4',
    barcodes: [{ value: 'A4-TEST-001', format: 'CODE128', primary: true }],
    unit: 'un',
    priceMinor: '0',
    sellable: false,
    stockMode: 'own',
    stockPolicy: 'block',
    components: [],
  });
  await post('/inventory/receipts', {
    productId: paper.id,
    quantity: '100.125',
    reason: 'Papel inicial',
  });
  const color = await post('/products', {
    name: 'Copia color',
    sku: 'COP-C',
    unit: 'un',
    priceMinor: '2000',
    sellable: true,
    stockMode: 'components',
    stockPolicy: 'block',
    components: [{ productId: paper.id, quantity: '1' }],
  });
  const bw = await post('/products', {
    name: 'Copia B/N',
    sku: 'COP-BN',
    unit: 'un',
    priceMinor: '1000',
    sellable: true,
    stockMode: 'components',
    stockPolicy: 'block',
    components: [{ productId: paper.id, quantity: '1' }],
  });
  const operator = await post('/users', {
    name: 'Pedro Operador',
    username: 'pedro',
    password: 'Otra frase segura 2026',
    role: 'operator',
    canApplyManualDiscounts: false,
  });
  await page.getByRole('link', { name: 'Descuentos y ofertas', exact: true }).click();
  await page.getByRole('button', { name: 'Nueva oferta', exact: true }).click();
  await page.getByLabel('Nombre de la oferta', { exact: true }).fill('Dos copias y una gratis');
  await page.getByLabel('Tipo de oferta', { exact: true }).selectOption('buy_get');
  const participants = page.getByRole('group', { name: 'Productos participantes', exact: true });
  await participants.getByRole('checkbox', { name: /Copia color/ }).check();
  await participants.getByRole('checkbox', { name: /Copia B\/N/ }).check();
  await page.getByLabel('Por cada unidades compradas', { exact: true }).fill('2');
  await page
    .getByRole('group', { name: 'Productos de regalo', exact: true })
    .getByRole('checkbox', { name: /Copia color/ })
    .check();
  await page.getByRole('button', { name: 'Guardar oferta', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText('Dos copias y una gratis', { exact: true })).toBeVisible();

  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await page.getByRole('button', { name: 'Nueva venta', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Agregar Hoja A4', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Agregar Copia color', exact: true }).click();
  const amount = page.getByLabel('Cantidad de Copia color', { exact: true });
  await amount.fill('1,25');
  await amount.press('ArrowUp');
  await expect(amount).toHaveValue('2.25');
  await amount.press('ArrowDown');
  await expect(amount).toHaveValue('1.25');
  await page.getByRole('button', { name: 'Aumentar Cantidad de Copia color', exact: true }).click();
  await expect(amount).toHaveValue('2.25');
  await amount.fill('2');
  await page.getByRole('button', { name: 'Agregar Copia B/N', exact: true }).click();
  await page.getByRole('button', { name: 'Agregar regalo', exact: true }).click();
  await expect(page.getByLabel('Cantidad gratis de Copia color', { exact: true })).toHaveValue('1');
  await page.getByRole('button', { name: 'Aplicar descuento a Copia color', exact: true }).click();
  await page.getByRole('dialog').getByLabel('Porcentaje', { exact: true }).fill('10');
  await page
    .getByRole('dialog')
    .getByLabel('Motivo del descuento', { exact: true })
    .fill('Cliente habitual');
  await page.screenshot({ path: 'test-results/discount-reason-dialog.png', fullPage: true });
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Aplicar descuento', exact: true })
    .click();
  async function totalDiscount(type: 'percentage' | 'amount', value: string, editing = false) {
    await page
      .getByRole('button', {
        name: editing ? 'Editar descuento total' : 'Descuento sobre el total',
        exact: true,
      })
      .click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Tipo de descuento', { exact: true }).selectOption(type);
    await dialog
      .getByLabel(type === 'amount' ? 'Monto (CLP)' : 'Porcentaje', { exact: true })
      .fill(value);
    await expect(dialog.getByLabel('Motivo del descuento', { exact: true })).toHaveValue(
      'Cliente habitual'
    );
    await dialog.getByRole('button', { name: 'Aplicar descuento', exact: true }).click();
    await expect(dialog).toHaveCount(0);
  }
  async function removeTotalDiscount() {
    await page.getByRole('button', { name: 'Quitar descuento total', exact: true }).click();
    await expect(page.locator('.cart-total')).toContainText('$ 4.600');
    await expect(page.locator('.discount-reason-summary')).toContainText('Cliente habitual');
    await expect(page.getByLabel('Cantidad gratis de Copia color', { exact: true })).toHaveValue(
      '1'
    );
    const draft = await page.evaluate(
      (id) => JSON.parse(sessionStorage.getItem(`raiz-cart:${id}`)!),
      session.user.id
    );
    expect(draft.totalDiscount).toBeUndefined();
    expect(draft.lineDiscounts).toHaveLength(1);
    expect(draft.lineDiscounts[0].productId).toBe(color.id);
    expect(draft.discountReason).toBe('Cliente habitual');
  }
  await totalDiscount('percentage', '10');
  await expect(page.locator('.cart-total')).toContainText('$ 4.140');
  await totalDiscount('percentage', '5', true);
  await expect(page.locator('.cart-total')).toContainText('$ 4.370');
  await removeTotalDiscount();
  await totalDiscount('amount', '100');
  await expect(page.locator('.cart-total')).toContainText('$ 4.500');
  await totalDiscount('amount', '200', true);
  await expect(page.locator('.cart-total')).toContainText('$ 4.400');
  await removeTotalDiscount();
  await totalDiscount('amount', '100');
  await expect(page.locator('.cart-total')).toContainText('$ 4.500');
  await expectTotalDiscountActions(page);
  await page.screenshot({
    path: 'test-results/total-discount-actions-desktop.png',
    fullPage: true,
  });
  const desktopViewport = page.viewportSize()!;
  await page.setViewportSize({ width: 390, height: 844 });
  await expectTotalDiscountActions(page);
  await page.screenshot({ path: 'test-results/total-discount-actions-mobile.png', fullPage: true });
  await page.setViewportSize({ width: 320, height: 740 });
  await expectTotalDiscountActions(page);
  await page.setViewportSize(desktopViewport);
  await page.route('**/api/v1/sales', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    await route.fetch();
    await route.abort();
  });
  await page.getByRole('button', { name: 'Confirmar venta', exact: true }).click();
  await expect(page.getByText(/La venta está pendiente de confirmación/)).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Editar descuento total', exact: true })
  ).toBeDisabled();
  await expect(
    page.getByRole('button', { name: 'Quitar descuento total', exact: true })
  ).toBeDisabled();
  await page.reload();
  await expect(page.locator('.discount-reason-summary')).toContainText('Cliente habitual');
  await expect(page.getByLabel('Cantidad gratis de Copia color', { exact: true })).toHaveValue('1');
  await expect(page.locator('.cart-total')).toContainText('$ 4.500');
  await page.unroute('**/api/v1/sales');
  await page.getByRole('button', { name: 'Reintentar confirmación', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('Venta registrada correctamente')).toBeVisible();
  await expect(page.getByRole('dialog')).toContainText('Cliente habitual');
  await page
    .getByRole('dialog')
    .getByText('Detalle interno de descuentos', { exact: true })
    .click();
  await expect(
    page.getByRole('dialog').getByText('Responsable: María Torres', { exact: true }).first()
  ).toBeVisible();
  await expect(page.getByRole('dialog')).toContainText('Gratis');
  const latest = await (await page.request.get('/api/v1/sales?pageSize=1')).json();
  const sale = latest.items[0];
  expect(sale.totalMinor).toBe('4500');
  expect(sale.lines).toHaveLength(3);
  const currentPaper = await (await page.request.get(`/api/v1/products/${paper.id}`)).json();
  expect(currentPaper.stock).toBe('96.125000');
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();

  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await page.getByRole('button', { name: 'Editar Copia color', exact: true }).click();
  await page.getByLabel('Nombre del producto', { exact: true }).fill('Copia color premium');
  await page.getByLabel('Precio de venta (CLP)', { exact: true }).fill('3000');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await page.getByRole('button', { name: `Ver venta ${sale.number}`, exact: true }).click();
  await expect(
    page.getByRole('dialog').getByText('Copia color', { exact: true }).first()
  ).toBeVisible();
  await expect(
    page.getByRole('dialog').getByText('Copia color premium', { exact: true })
  ).toHaveCount(0);
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();
  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await page.getByRole('button', { name: 'Ajustar existencias de Hoja A4', exact: true }).click();
  const counted = page.getByLabel('Existencias contadas (un)', { exact: true });
  await counted.fill('100,125001');
  await counted.press('ArrowUp');
  await expect(counted).toHaveValue('101.125001');
  await page.getByLabel('Motivo del ajuste', { exact: true }).fill('Conteo físico verificado');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  await page.getByRole('link', { name: 'Administración', exact: true }).click();
  const permission = page.getByRole('checkbox', {
    name: 'Permitir descuentos a Pedro Operador',
    exact: true,
  });
  await permission.check();
  await expect(permission).toBeEnabled();
  expect(
    (await (await page.request.get('/api/v1/users')).json()).items.find(
      (u: { id: string }) => u.id === operator.id
    ).canApplyManualDiscounts
  ).toBe(true);
  await permission.uncheck();
  await expect(permission).toBeEnabled();
  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await page.getByRole('button', { name: 'Nueva venta', exact: true }).click();
  await page.getByRole('button', { name: 'Agregar Copia color premium', exact: true }).click();
  await page.getByLabel('Cantidad de Copia color premium', { exact: true }).fill('1.25');
  await page.getByRole('link', { name: 'Ajustes', exact: true }).click();
  await page.getByLabel('Nombre visible', { exact: true }).fill('María Administradora');
  await page.getByRole('button', { name: 'Guardar perfil', exact: true }).click();
  await expect(page.getByText('Perfil actualizado.', { exact: true })).toBeVisible();
  await page.getByLabel('Nombre de empresa', { exact: true }).fill('Centro de copiado');
  await page.getByLabel('Zona horaria', { exact: true }).fill('America/Santiago');
  await page.getByRole('button', { name: 'Guardar empresa', exact: true }).click();
  await expect(page.getByText('Datos de la empresa actualizados.', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Cambiar de usuario', exact: true }).click();
  await login('pedro', 'Otra frase segura 2026');
  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await expect(
    page.getByRole('columnheader', { name: 'Precio de venta', exact: true })
  ).toBeVisible();
  await expect(page.getByRole('button', { name: 'Nuevo producto', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^Ingresar existencias de/ })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^Ajustar existencias de/ })).toHaveCount(0);
  await page.keyboard.press('F2');
  const productScan = page.getByLabel('Escanear en productos', { exact: true });
  await expect(productScan).toBeFocused();
  await productScan.fill('A4-TEST-001');
  await productScan.press('Enter');
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Hoja A4', exact: true })
  ).toBeVisible();
  await expect(
    page.getByRole('dialog').getByRole('button', { name: 'Editar producto', exact: true })
  ).toHaveCount(0);
  await page.getByRole('dialog').getByRole('button', { name: 'Cerrar', exact: true }).click();
  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await page.getByRole('button', { name: 'Nueva venta', exact: true }).click();
  await expect(page.getByLabel('Cantidad de Copia color premium', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Agregar Copia B/N', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Descuento sobre el total', exact: true })
  ).toHaveCount(0);
  await page.getByRole('link', { name: 'Ajustes', exact: true }).click();
  await page.getByRole('button', { name: 'Cambiar de usuario', exact: true }).click();
  await login();
  await page.getByRole('link', { name: 'Ventas', exact: true }).click();
  await page.getByRole('button', { name: 'Nueva venta', exact: true }).click();
  await expect(page.getByLabel('Cantidad de Copia color premium', { exact: true })).toHaveValue(
    '1.25'
  );
  await expect(page.getByLabel('Cantidad de Copia B/N', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Confirmar venta', exact: true })).toBeEnabled();
  await page.screenshot({ path: 'test-results/promotions-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: 'test-results/promotions-mobile.png', fullPage: true });
  await page.setViewportSize({ width: 1440, height: 1000 });

  const other = await browser.newContext({ baseURL: base });
  try {
    await other.request.post('/api/v1/auth/login', {
      data: { username: 'maria', password: 'Una frase segura 2026' },
    });
    expect((await other.request.get('/api/v1/auth/me')).status()).toBe(200);
    await page.getByRole('link', { name: 'Ajustes', exact: true }).click();
    await page.getByLabel('Contraseña actual', { exact: true }).fill('Una frase segura 2026');
    await page.getByLabel('Nueva contraseña', { exact: true }).fill('Mi nueva frase segura 2026');
    await page
      .getByLabel('Confirmar nueva contraseña', { exact: true })
      .fill('Mi nueva frase segura 2026');
    await page.getByRole('button', { name: 'Actualizar contraseña', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Iniciar sesión', exact: true })).toBeVisible();
    expect((await other.request.get('/api/v1/auth/me')).status()).toBe(401);
    await login('maria', 'Mi nueva frase segura 2026');
  } finally {
    await other.close();
  }
  expect(errors).toEqual([]);
});

test('códigos exportables, lector contextual, auditoría detallada y cierre al imprimir', async ({
  page,
  context,
}) => {
  test.setTimeout(90_000);
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  const base = process.env.ERP_BASE_URL ?? 'http://127.0.0.1:8080';
  await context.route('**/*', (route) =>
    new URL(route.request().url()).origin === new URL(base).origin
      ? route.continue()
      : route.abort()
  );
  await loginCurrent(page);
  const session = await (await page.request.get('/api/v1/auth/me')).json();
  await page.evaluate((id) => sessionStorage.removeItem(`raiz-cart:${id}`), session.user.id);
  await page.evaluate(() => {
    location.hash = '/inventory';
  });
  await expect(page).toHaveURL(/#\/products$/);
  await page.keyboard.press('F2');
  await expect(page.getByLabel('Escanear en productos', { exact: true })).toBeFocused();
  await page.getByRole('button', { name: 'Nuevo producto', exact: true }).click();
  await page.getByLabel('Nombre del producto', { exact: true }).fill('Marcador azul');
  await page.getByLabel('Código SKU', { exact: true }).fill('V3-BLUE');
  await page.getByLabel('Precio de venta (CLP)', { exact: true }).fill('250');
  await page.getByLabel('Control de existencias', { exact: true }).selectOption('untracked');
  await page.getByRole('button', { name: 'Añadir código', exact: true }).click();
  await page.getByLabel('Código de barras 1', { exact: true }).fill('4006381333931');
  await page.getByLabel('Formato 1', { exact: true }).selectOption('EAN13');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('button', { name: 'Ver producto Marcador azul', exact: true }).click();
  await page.getByRole('button', { name: 'Generar código interno', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Consultar código interno', exact: true })
  ).toBeEnabled();
  const p = await (await page.request.get('/api/v1/products/lookup?barcode=4006381333931')).json();
  expect(p.barcodes).toHaveLength(2);
  const generated = p.barcodes.find((b: { generated: boolean }) => b.generated).value;
  await page.getByRole('button', { name: 'Consultar código interno', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Consultar código interno', exact: true })
  ).toBeEnabled();
  expect((await (await page.request.get(`/api/v1/products/${p.id}`)).json()).barcodes).toHaveLength(
    2
  );
  const card = page.locator('.barcode-card').filter({
    has: page.getByRole('checkbox', { name: `Seleccionar código ${generated}`, exact: true }),
  });
  let downloaded = page.waitForEvent('download');
  await card.getByRole('button', { name: 'SVG', exact: true }).click();
  let file = await downloaded;
  const svg = readFileSync((await file.path())!, 'utf8');
  expect(svg).toContain(generated);
  expect(svg).toContain('<rect');
  downloaded = page.waitForEvent('download');
  await card.getByRole('button', { name: 'PNG', exact: true }).click();
  file = await downloaded;
  const png = readFileSync((await file.path())!);
  expect(png.subarray(0, 8).toString('hex')).toBe('89504e470d0a1a0a');
  expect(png.readUInt32BE(16)).toBeGreaterThan(200);
  await page
    .getByRole('checkbox', { name: `Seleccionar código ${generated}`, exact: true })
    .check();
  await page
    .getByRole('checkbox', { name: 'Seleccionar código 4006381333931', exact: true })
    .check();
  downloaded = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Exportar selección ZIP', exact: true }).click();
  file = await downloaded;
  const entries = unzipSync(readFileSync((await file.path())!));
  expect(Object.keys(entries)).toHaveLength(2);
  expect(Object.values(entries).every((v) => strFromU8(v).includes('<svg'))).toBe(true);
  await page.getByRole('dialog').getByRole('button', { name: 'Cerrar', exact: true }).click();
  await page.keyboard.press('F2');
  const catalogScan = page.getByLabel('Escanear en productos', { exact: true });
  await expect(catalogScan).toBeFocused();
  await catalogScan.fill(generated);
  await catalogScan.press('Enter');
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Marcador azul', exact: true })
  ).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Cerrar', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Productos', exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Resumen', exact: true }).click();
  await page.keyboard.type('4006381333931', { delay: 3 });
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { name: 'Nueva venta', exact: true })).toBeVisible();
  const qty = page.getByLabel('Cantidad de Marcador azul', { exact: true });
  await expect(qty).toHaveValue('1');
  await page.locator('#main-content').focus();
  await page.keyboard.type('A'.repeat(65) + '4006381333931', { delay: 1 });
  await page.keyboard.press('Enter');
  await expect(
    page.getByText('El código supera los 64 caracteres. Revisa la configuración del lector.', {
      exact: true,
    })
  ).toBeVisible();
  await expect(qty).toHaveValue('1');
  const scan = page.getByLabel('Escanear código de barras', { exact: true });
  await scan.fill(generated);
  await scan.press('Enter');
  await scan.fill('4006381333931');
  await scan.press('Enter');
  await expect(qty).toHaveValue('3');
  await scan.fill('BLUE-ALT-001');
  await scan.press('Enter');
  await expect(page.getByRole('dialog', { name: 'Código sin registrar' })).toBeVisible();
  await page.getByRole('button', { name: 'Vincular a un producto existente', exact: true }).click();
  await page.getByLabel('Buscar producto para vincular', { exact: true }).fill('V3-BLUE');
  await page.getByRole('button', { name: /Marcador azul V3-BLUE Vincular/ }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await scan.fill('BLUE-ALT-001');
  await scan.press('Enter');
  await expect(qty).toHaveValue('4');
  await page.getByRole('button', { name: 'Confirmar venta', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('Venta registrada correctamente')).toBeVisible();
  await expect(page.getByRole('dialog')).toContainText('$ 1.000');
  await page.evaluate(() => {
    window.print = () => window.dispatchEvent(new Event('afterprint'));
  });
  await page.getByRole('dialog').getByRole('button', { name: 'Imprimir', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('link', { name: 'Productos', exact: true }).click();
  await page.getByRole('button', { name: 'Editar Marcador azul', exact: true }).click();
  await page.getByLabel('Nombre del producto', { exact: true }).fill('Marcador azul nuevo');
  await page.getByLabel('Precio de venta (CLP)', { exact: true }).fill('350');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('link', { name: 'Auditoría', exact: true }).click();
  const filter = page.getByRole('combobox', { name: 'Buscar operación', exact: true });
  await filter.fill('Ed');
  await expect(
    page.getByRole('option', { name: 'Edición de producto', exact: true })
  ).toBeVisible();
  await page.getByRole('option', { name: 'Edición de producto', exact: true }).click();
  const row = page.getByRole('row').filter({ hasText: 'Marcador azul nuevo' });
  await row
    .getByRole('button', {
      name: 'Ver detalle de Edición de producto Producto V3-BLUE',
      exact: true,
    })
    .click();
  let detail = page.getByRole('dialog');
  await expect(detail.getByRole('columnheader', { name: 'Antes', exact: true })).toBeVisible();
  await expect(detail.getByRole('row').filter({ hasText: 'Nombre' })).toContainText(
    'Marcador azul'
  );
  await expect(detail.getByRole('row').filter({ hasText: 'Precio de venta' })).toContainText(
    '$ 250'
  );
  await expect(detail.getByRole('row').filter({ hasText: 'Precio de venta' })).toContainText(
    '$ 350'
  );
  await page.screenshot({ path: 'test-results/audit-detail-desktop.png', fullPage: true });
  await detail.getByRole('button', { name: 'Ver producto actual', exact: true }).click();
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Marcador azul nuevo', exact: true })
  ).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Cerrar', exact: true }).click();
  await page.getByRole('button', { name: 'Limpiar filtro de auditoría', exact: true }).click();
  await filter.fill('confirmacion');
  await filter.press('Enter');
  await expect(filter).toHaveValue('Confirmación de venta');
  await page
    .getByRole('row')
    .filter({ hasText: 'Marcador azul × 4.000' })
    .getByRole('button', { name: /Ver detalle de Confirmación de venta/ })
    .click();
  detail = page.getByRole('dialog');
  await expect(detail.getByRole('cell', { name: 'Marcador azul', exact: true })).toBeVisible();
  await expect(detail.getByText('Marcador azul nuevo', { exact: true })).toHaveCount(0);
  await detail.getByRole('button', { name: 'Ver comprobante de venta', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('Venta registrada correctamente')).toBeVisible();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: 'test-results/audit-mobile.png', fullPage: true });
  expect(errors).toEqual([]);
});

test('configuración de impresora y verificación física guiada con transporte simulado', async ({
  page,
}) => {
  await loginCurrent(page);
  const session = await (await page.request.get('/api/v1/auth/me')).json();
  let config = {
    version: 0,
    deviceId: '',
    deviceName: '',
    width: 80,
    cutter: false,
    drawerPin: 0,
    pulseOnMs: 100,
    pulseOffMs: 500,
    enabled: false,
    drawerEnabled: false,
    printVerified: false,
    drawerVerified: false,
  };
  const jobs: Record<string, unknown>[] = [];
  await page.route('**/api/v1/pos/**', async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname;
    const reply = (body: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
    if (path.endsWith('/devices'))
      return reply([{ id: 'usb-qa', name: 'Xprinter simulada', transport: 'fake' }]);
    if (path.endsWith('/settings')) {
      if (req.method() === 'PUT') {
        expect(req.headers()['x-csrf-token']).toBeTruthy();
        config = { ...req.postDataJSON(), version: config.version + 1 };
      }
      return reply(config);
    }
    if (path.endsWith('/tests')) {
      expect(req.headers()['idempotency-key']).toBeTruthy();
      const kind = req.postDataJSON().kind;
      const job = {
        id: String(jobs.length + 1).padStart(32, 'a'),
        kind,
        state: 'sent',
        actorId: session.user.id,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        opensDrawer: kind === 'test_drawer',
        reason: '',
        error: '',
        spoolId: 'qa',
        printerName: 'Xprinter simulada',
      };
      jobs.push(job);
      return reply(job, 202);
    }
    if (path.endsWith('/verify')) {
      const job = jobs.find((j) => j.id === req.postDataJSON().jobId)!;
      config = {
        ...config,
        version: config.version + 1,
        printVerified: config.printVerified || job.kind === 'test_print',
        drawerVerified: config.drawerVerified || job.kind === 'test_drawer',
      };
      return reply(config);
    }
    if (path.endsWith('/jobs'))
      return reply({ items: jobs, total: jobs.length, page: 1, pageSize: 20 });
    if (path.includes('/jobs/')) return reply(jobs.find((j) => j.id === path.split('/').at(-1)));
    return route.continue();
  });
  await page.getByRole('link', { name: 'Ajustes', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Periféricos de caja', exact: true })
  ).toBeVisible();
  const auto = page.getByRole('checkbox', {
    name: 'Imprimir automáticamente cada venta nueva',
    exact: true,
  });
  await expect(auto).toBeDisabled();
  await page.getByLabel('Impresora del servidor', { exact: true }).selectOption('usb-qa');
  await expect(
    page.getByRole('button', { name: 'Imprimir ticket de prueba', exact: true })
  ).toBeDisabled();
  await page.getByRole('button', { name: 'Guardar periféricos', exact: true }).click();
  await page.getByRole('button', { name: 'Imprimir ticket de prueba', exact: true }).click();
  await page
    .getByRole('button', { name: 'Confirmar que el ticket salió correctamente', exact: true })
    .click();
  await expect(auto).toBeEnabled();
  await page.getByRole('button', { name: 'Probar apertura del cajón', exact: true }).click();
  await page
    .getByRole('button', { name: 'Confirmar que el cajón abrió correctamente', exact: true })
    .click();
  await auto.check();
  await page
    .getByRole('checkbox', {
      name: 'Abrir el cajón al confirmar ventas y registrar egresos',
      exact: true,
    })
    .check();
  await page.getByRole('button', { name: 'Guardar periféricos', exact: true }).click();
  await expect(auto).toBeChecked();
  await expect(
    page.getByRole('checkbox', {
      name: 'Abrir el cajón al confirmar ventas y registrar egresos',
      exact: true,
    })
  ).toBeChecked();
  await page.getByLabel('Ancho del papel', { exact: true }).selectOption('58');
  await expect(auto).not.toBeChecked();
  await expect(auto).toBeDisabled();
  await expect(
    page.getByRole('button', { name: 'Imprimir ticket de prueba', exact: true })
  ).toBeDisabled();
});

test('egresos recuperables, permisos y contraseñas cortas o vacías', async ({ page, browser }) => {
  test.setTimeout(150_000);
  // Earlier scenarios exercise account switching. Start a fresh one-minute
  // window for this authentication scenario without relaxing the real limiter.
  await page.waitForTimeout(60_000);
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await loginCurrent(page);
  await page.getByRole('link', { name: 'Caja', exact: true }).click();
  const before = await (await page.request.get('/api/v1/reports/summary')).json();
  const amount = (BigInt(before.cashMinor) + 123n).toString();
  await page.getByRole('button', { name: 'Registrar egreso', exact: true }).click();
  const form = page.getByRole('dialog');
  await form.getByLabel('Monto (CLP)', { exact: true }).fill(amount);
  await form.getByLabel('Motivo del egreso', { exact: true }).fill('Compra de papel higiénico');
  await expect(form).toContainText('Quedará negativo');
  const attempts: { key: string; body: string | null }[] = [];
  await page.route('**/api/v1/cash/expenses', async (route) => {
    attempts.push({
      key: route.request().headers()['idempotency-key'],
      body: route.request().postData(),
    });
    await route.fetch();
    await route.abort();
  });
  await form.getByRole('button', { name: 'Registrar egreso', exact: true }).click();
  await expect(form).toContainText('El egreso está pendiente de confirmación');
  await expect(form.getByRole('alert')).toBeVisible();
  await page.unroute('**/api/v1/cash/expenses');
  await page.route('**/api/v1/cash/expenses', async (route) => {
    attempts.push({
      key: route.request().headers()['idempotency-key'],
      body: route.request().postData(),
    });
    await route.continue();
  });
  await page.reload();
  await expect(page.getByRole('dialog').getByLabel('Motivo del egreso')).toHaveValue(
    'Compra de papel higiénico'
  );
  await page.getByRole('button', { name: 'Reintentar registro', exact: true }).click();
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Egreso #00001', exact: true })
  ).toBeVisible();
  await expect(page.getByRole('dialog')).toContainText('Saldo después del egreso');
  await expect(page.getByRole('dialog')).toContainText('$ −123');
  expect(attempts).toHaveLength(2);
  expect(attempts[1]).toEqual(attempts[0]);
  const expenses = await (await page.request.get('/api/v1/cash/movements?kind=expense')).json();
  expect(expenses.total).toBe(1);
  expect(expenses.items[0].amountMinor).toBe(`-${amount}`);
  const after = await (await page.request.get('/api/v1/reports/summary')).json();
  expect(after.cashMinor).toBe('-123');
  expect(after.revenueMinor).toBe(before.revenueMinor);
  await page.screenshot({ path: 'test-results/cash-expense-detail.png', fullPage: true });
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();
  await page.getByLabel('Tipo de movimiento', { exact: true }).selectOption('expense');
  await page.getByPlaceholder('Buscar referencia o motivo').fill('higiénico');
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await page.getByRole('button', { name: 'Egreso #00001', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Compra de papel higiénico');
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();
  await page.getByRole('link', { name: 'Auditoría', exact: true }).click();
  await page.getByRole('combobox', { name: 'Buscar operación', exact: true }).fill('egreso');
  await page.getByRole('option', { name: 'Registro de egreso de caja', exact: true }).click();
  await page
    .getByRole('button', {
      name: 'Ver detalle de Registro de egreso de caja Egreso #00001',
      exact: true,
    })
    .click();
  await expect(page.getByRole('dialog')).toContainText('Compra de papel higiénico');
  await page.getByRole('dialog').getByRole('button', { name: 'Ver egreso', exact: true }).click();
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Egreso #00001', exact: true })
  ).toBeVisible();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Cerrar', exact: true })
    .last()
    .click();

  await page.getByRole('link', { name: 'Administración', exact: true }).click();
  await page.getByRole('button', { name: 'Nuevo usuario', exact: true }).click();
  await page
    .getByRole('dialog')
    .getByLabel('Nombre completo', { exact: true })
    .fill('Cuenta sin clave');
  await page.getByRole('dialog').getByLabel('Usuario', { exact: true }).fill('sinclave');
  await expect(page.getByRole('dialog')).toContainText('podrá entrar sin contraseña');
  await page
    .getByRole('dialog')
    .getByRole('checkbox', { name: 'Usar esta contraseña aunque sea poco segura', exact: true })
    .check();
  await page.getByRole('dialog').getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  const expensePermission = page.getByRole('checkbox', {
    name: 'Permitir egresos a Cuenta sin clave',
    exact: true,
  });
  await expect(expensePermission).not.toBeChecked();
  await expensePermission.check();
  await expect(expensePermission).toBeEnabled();
  await page.getByRole('button', { name: 'Cerrar sesión', exact: true }).click();
  await page.getByLabel('Usuario', { exact: true }).fill('sinclave');
  await page.getByRole('button', { name: 'Iniciar sesión', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Caja', exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Caja', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Registrar egreso', exact: true })).toBeVisible();
  const baseURL = process.env.ERP_BASE_URL ?? 'http://127.0.0.1:8080';
  const other = await browser.newContext({ baseURL });
  try {
    expect(
      (
        await other.request.post('/api/v1/auth/login', {
          data: { username: 'sinclave', password: '' },
        })
      ).status()
    ).toBe(200);
    await page.getByRole('link', { name: 'Ajustes', exact: true }).click();
    await page.getByLabel('Nueva contraseña', { exact: true }).fill('1');
    await page.getByLabel('Confirmar nueva contraseña', { exact: true }).fill('1');
    await page
      .getByRole('checkbox', { name: 'Usar esta contraseña aunque sea poco segura', exact: true })
      .check();
    await page.getByRole('button', { name: 'Actualizar contraseña', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Iniciar sesión', exact: true })).toBeVisible();
    expect((await other.request.get('/api/v1/auth/me')).status()).toBe(401);
    await page.getByLabel('Usuario', { exact: true }).fill('sinclave');
    await page.getByLabel('Contraseña', { exact: true }).fill('1');
    await page.getByRole('button', { name: 'Iniciar sesión', exact: true }).click();
    await page.getByRole('link', { name: 'Ajustes', exact: true }).click();
    await page.getByLabel('Contraseña actual', { exact: true }).fill('1');
    await page
      .getByRole('checkbox', { name: 'Usar esta contraseña aunque sea poco segura', exact: true })
      .check();
    await page.getByRole('button', { name: 'Actualizar contraseña', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Iniciar sesión', exact: true })).toBeVisible();
    await page.getByLabel('Usuario', { exact: true }).fill('sinclave');
    await page.getByRole('button', { name: 'Iniciar sesión', exact: true }).click();
    await expect(page.getByRole('link', { name: 'Caja', exact: true })).toBeVisible();
  } finally {
    await other.close();
  }
  await page.getByRole('button', { name: 'Cerrar sesión', exact: true }).click();
  await loginCurrent(page);
  await page.getByRole('link', { name: 'Caja', exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: 'test-results/cash-expenses-mobile.png', fullPage: true });
  expect(errors).toEqual([]);
});
