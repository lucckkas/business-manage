import { useRef, useState } from 'react';
import JsBarcode from 'jsbarcode';
import { strToU8, zipSync } from 'fflate';
import { Download, ScanBarcode } from 'lucide-react';
import { api, type Product, type Schema } from './api/client';
import { ErrorMessage, Field } from './ui';
type Barcode = Schema['Barcode'];

function svgFor(code: Barcode) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  JsBarcode(svg, code.value, {
    format: code.format ?? 'CODE128',
    width: 2,
    height: 80,
    margin: 20,
    fontSize: 16,
    displayValue: true,
    background: '#ffffff',
    lineColor: '#000000',
  });
  svg.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  return new XMLSerializer().serializeToString(svg);
}
function download(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob),
    link = document.createElement('a');
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
async function exportCode(code: Barcode, kind: 'svg' | 'png') {
  const svg = svgFor(code);
  const name = code.value.replace(/[^a-zA-Z0-9_-]/g, '_');
  if (kind === 'svg') {
    download(new Blob([svg], { type: 'image/svg+xml' }), `${name}.svg`);
    return;
  }
  const canvas = document.createElement('canvas');
  JsBarcode(canvas, code.value, {
    format: code.format ?? 'CODE128',
    width: 2,
    height: 80,
    margin: 20,
    fontSize: 16,
    displayValue: true,
    background: '#ffffff',
    lineColor: '#000000',
  });
  const blob = await new Promise<Blob>((resolve, reject) =>
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('No se pudo generar la imagen.'))),
      'image/png'
    )
  );
  download(blob, `${name}.png`);
}
export function BarcodeInputs({
  codes,
  onChange,
}: {
  codes: Barcode[];
  onChange: (codes: Barcode[]) => void;
}) {
  return (
    <fieldset className="barcode-inputs">
      <legend>Códigos de barras</legend>
      <p className="form-note">
        Conserva el código del fabricante y añade otros para el mismo producto. El SKU es
        independiente.
      </p>
      {codes.map((code, index) => (
        <div className="barcode-input-row" key={index}>
          <Field label={`Código de barras ${index + 1}`}>
            <input
              required
              maxLength={64}
              value={code.value}
              readOnly={code.generated}
              autoComplete="off"
              onChange={(event) =>
                onChange(
                  codes.map((b, i) => (i === index ? { ...b, value: event.target.value } : b))
                )
              }
              onKeyDown={(event) => {
                if (event.key === 'Enter') event.preventDefault();
              }}
            />
          </Field>
          <Field label={`Formato ${index + 1}`}>
            <select
              value={code.format ?? 'CODE128'}
              disabled={code.generated}
              onChange={(event) =>
                onChange(
                  codes.map((b, i) =>
                    i === index ? { ...b, format: event.target.value as Barcode['format'] } : b
                  )
                )
              }
            >
              <option value="CODE128">CODE128</option>
              <option value="EAN8">EAN-8</option>
              <option value="EAN13">EAN-13</option>
              <option value="UPC">UPC-A</option>
            </select>
          </Field>
          <label className="barcode-primary">
            <input
              type="radio"
              name="barcode-primary"
              checked={!!code.primary}
              onChange={() => onChange(codes.map((b, i) => ({ ...b, primary: i === index })))}
            />
            Principal
          </label>
          <button
            type="button"
            className="text-button"
            aria-label={`Quitar código ${index + 1}`}
            onClick={() => {
              const next = codes.filter((_, i) => i !== index);
              if (next.length && !next.some((b) => b.primary))
                next[0] = { ...next[0], primary: true };
              onChange(next);
            }}
          >
            Quitar
          </button>
        </div>
      ))}
      <button
        type="button"
        className="button secondary"
        disabled={codes.length >= 30}
        onClick={() =>
          onChange([
            ...codes,
            { value: '', format: 'CODE128', primary: !codes.length, generated: false },
          ])
        }
      >
        Añadir código
      </button>
    </fieldset>
  );
}
export function BarcodeManager({
  product,
  admin,
  onChange,
}: {
  product: Product;
  admin: boolean;
  onChange: () => void;
}) {
  const [current, setCurrent] = useState(product),
    [selected, setSelected] = useState<string[]>([]),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  const preview = useRef<HTMLDivElement>(null);
  async function generate() {
    setBusy(true);
    setError('');
    try {
      const p = await api<Product>(`/products/${current.id}/barcodes/generate`, 'POST', {});
      setCurrent(p);
      onChange();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function exportZIP() {
    try {
      const files: Record<string, Uint8Array> = {};
      current.barcodes
        .filter((b) => selected.includes(b.value))
        .forEach((b, i) => {
          files[`${i + 1}-${b.value.replace(/[^a-zA-Z0-9_-]/g, '_')}.svg`] = strToU8(svgFor(b));
        });
      download(
        new Blob([zipSync(files).slice().buffer as ArrayBuffer], { type: 'application/zip' }),
        'codigos-de-barras.zip'
      );
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <section className="barcode-manager">
      <h3>
        <ScanBarcode size={18} />
        Códigos de barras
      </h3>
      <ErrorMessage message={error} />
      <p className="form-note">
        Descarga los archivos y elige dónde y cómo imprimirlos. Conserva los márgenes blancos al
        imprimir.
      </p>
      {current.barcodes.length ? (
        <div ref={preview}>
          {current.barcodes.map((code) => (
            <div className="barcode-card" key={code.value}>
              <label>
                <input
                  type="checkbox"
                  aria-label={`Seleccionar código ${code.value}`}
                  checked={selected.includes(code.value)}
                  onChange={(e) =>
                    setSelected(
                      e.target.checked
                        ? [...selected, code.value]
                        : selected.filter((v) => v !== code.value)
                    )
                  }
                />
                {code.value} {code.primary && <span className="badge neutral">Principal</span>}
              </label>
              <BarcodePreview code={code} />
              <div className="row-actions">
                <button
                  className="text-button"
                  onClick={() => exportCode(code, 'svg').catch((e) => setError(e.message))}
                >
                  <Download size={14} />
                  SVG
                </button>
                <button
                  className="text-button"
                  onClick={() => exportCode(code, 'png').catch((e) => setError(e.message))}
                >
                  <Download size={14} />
                  PNG
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p className="form-note">Este producto todavía no tiene códigos asociados.</p>
      )}
      <div className="row-actions">
        {admin && (
          <button className="button secondary" disabled={busy} onClick={generate}>
            {busy
              ? 'Generando…'
              : current.barcodes.some((b) => b.generated)
                ? 'Consultar código interno'
                : 'Generar código interno'}
          </button>
        )}
        <button className="button secondary" disabled={!selected.length} onClick={exportZIP}>
          <Download size={15} />
          Exportar selección ZIP
        </button>
      </div>
    </section>
  );
}
function BarcodePreview({ code }: { code: Barcode }) {
  try {
    const svg = svgFor(code);
    return (
      <div
        className="barcode-preview"
        role="img"
        aria-label={`Código ${code.value}`}
        dangerouslySetInnerHTML={{ __html: svg }}
      />
    );
  } catch {
    return (
      <p className="error-message">
        No se puede representar este código con el formato seleccionado.
      </p>
    );
  }
}
