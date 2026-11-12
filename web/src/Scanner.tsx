import { useEffect, useRef, useState } from 'react';
import { ScanBarcode } from 'lucide-react';

type Receiver = (code: string) => Promise<void>;
const backlog: string[] = [];
let receiver: Receiver | null = null;
let draining = false;
export function resetScanner() {
  backlog.length = 0;
}
export function scanCode(code: string) {
  code = code.trim();
  if (!code || code.length > 64) return;
  backlog.push(code);
  void drain();
}
async function drain() {
  if (draining) return;
  draining = true;
  try {
    while (receiver && backlog.length) {
      const code = backlog.shift()!;
      try {
        await receiver(code);
      } catch {
        /* The receiver shows its error without dropping later readings. */
      }
    }
  } finally {
    draining = false;
  }
}
export function useScanReceiver(onScan: Receiver) {
  const latest = useRef(onScan);
  latest.current = onScan;
  useEffect(() => {
    const consume: Receiver = (code) => latest.current(code);
    receiver = consume;
    void drain();
    return () => {
      if (receiver === consume) receiver = null;
    };
  }, []);
}
export function useGlobalScanner(active: boolean, activate: () => void) {
  const latest = useRef(activate);
  latest.current = activate;
  useEffect(() => {
    if (!active) return;
    let buffer = '',
      last = 0,
      overflow = false;
    function key(event: KeyboardEvent) {
      if (document.querySelector('dialog[open]')) {
        buffer = '';
        overflow = false;
        return;
      }
      if (event.key === 'F2') {
        event.preventDefault();
        latest.current();
        requestAnimationFrame(() =>
          document.querySelector<HTMLInputElement>('[data-scanner]')?.focus()
        );
        return;
      }
      const target = event.target as HTMLElement | null;
      if (
        event.ctrlKey ||
        event.altKey ||
        event.metaKey ||
        event.isComposing ||
        event.repeat ||
        target?.closest('input,textarea,select,[contenteditable="true"]')
      ) {
        buffer = '';
        overflow = false;
        return;
      }
      const now = performance.now();
      if (now - last > 80) {
        buffer = '';
        overflow = false;
      }
      if (event.key === 'Enter') {
        if (overflow) {
          event.preventDefault();
          window.dispatchEvent(
            new CustomEvent('scanner-error', {
              detail: 'El código supera los 64 caracteres. Revisa la configuración del lector.',
            })
          );
          buffer = '';
          overflow = false;
          return;
        }
        if (buffer.length >= 3) {
          event.preventDefault();
          latest.current();
          scanCode(buffer);
        }
        buffer = '';
        return;
      }
      if (event.key.length === 1) {
        if (!overflow) buffer += event.key;
        if (buffer.length > 64) {
          buffer = '';
          overflow = true;
        }
        last = now;
      }
    }
    document.addEventListener('keydown', key);
    return () => document.removeEventListener('keydown', key);
  }, [active]);
}
export default function ScanField({
  label = 'Escanear código de barras',
  disabled = false,
}: {
  label?: string;
  disabled?: boolean;
}) {
  const [value, setValue] = useState('');
  return (
    <div className="scan-field">
      <ScanBarcode size={19} />
      <input
        data-scanner
        aria-label={label}
        value={value}
        disabled={disabled}
        placeholder="Escanea un código y pulsa Enter · F2"
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        onChange={(event) => {
          event.target.setCustomValidity('');
          setValue(event.target.value);
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            event.stopPropagation();
            if (value.trim().length > 64) {
              event.currentTarget.setCustomValidity('El código admite hasta 64 caracteres.');
              event.currentTarget.reportValidity();
              return;
            }
            if (!disabled) {
              scanCode(value);
              setValue('');
            }
          }
        }}
      />
      <kbd>F2</kbd>
    </div>
  );
}
