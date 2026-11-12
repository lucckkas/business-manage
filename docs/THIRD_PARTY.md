# Dependencias y distribución

El servidor usa Go, `golang.org/x/crypto` para Argon2id, `golang.org/x/sys` para bloqueos/servicio y APIs de impresión Windows, y `modernc.org/sqlite` con sus dependencias transitivas. La interfaz incorpora React, React DOM, scheduler, lucide-react, JsBarcode para SVG/PNG y fflate para ZIP. Vite, TypeScript, openapi-typescript y Playwright se utilizan en desarrollo y validación.

Los paquetes incluyen los textos de licencia disponibles en la raíz de los módulos Go, la licencia de Go y las licencias de los paquetes de interfaz incorporados, dentro de `licenses/`. Las versiones y checksums de origen están fijados en `go.mod`, `go.sum` y `web/package-lock.json`.

No se ha seleccionado una licencia para el código propio del proyecto. El titular del proyecto debe definirla antes de ofrecer su distribución a terceros. Las licencias de dependencias no conceden por sí mismas una licencia sobre el código propio.
