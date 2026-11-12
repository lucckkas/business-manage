import { useState, type InputHTMLAttributes } from 'react';
import { Minus, Plus } from 'lucide-react';
export function decimal(value: string, precision: number): bigint {
  value = value.replace(',', '.');
  const pattern = precision === 0 ? /^-?\d+$/ : new RegExp(`^-?\\d+(?:\\.\\d{1,${precision}})?$`);
  if (!pattern.test(value)) throw new Error('Valor decimal inválido.');
  const negative = value.startsWith('-');
  if (negative) value = value.slice(1);
  const [whole, fraction = ''] = value.split('.');
  const result =
    BigInt(whole) * 10n ** BigInt(precision) + BigInt(fraction.padEnd(precision, '0') || '0');
  return negative ? -result : result;
}
export function decimalText(value: bigint, precision: number): string {
  const sign = value < 0n ? '-' : '';
  if (value < 0n) value = -value;
  const factor = 10n ** BigInt(precision);
  const fraction = (value % factor).toString().padStart(precision, '0').replace(/0+$/, '');
  return `${sign}${value / factor}${fraction ? `.${fraction}` : ''}`;
}
type Props = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type' | 'value' | 'defaultValue' | 'onChange' | 'min' | 'max'
> & {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  precision?: number;
  minimum?: string;
  maximum?: string;
  label?: string;
};
export default function NumericInput({
  value,
  defaultValue = '',
  onValueChange,
  precision = 3,
  minimum = '0',
  maximum = '1000000000',
  label = 'valor',
  disabled,
  ...props
}: Props) {
  const [local, setLocal] = useState(defaultValue);
  const current = value ?? local;
  const set = (next: string) => {
    if (onValueChange) onValueChange(next);
    else setLocal(next);
  };
  function next(direction: number) {
    try {
      const old = current === '' ? 0n : decimal(current, precision);
      const candidate = old + BigInt(direction) * 10n ** BigInt(precision);
      if (candidate < decimal(minimum, precision) || candidate > decimal(maximum, precision))
        return null;
      return decimalText(candidate, precision);
    } catch {
      return null;
    }
  }
  function move(direction: number) {
    const result = next(direction);
    if (result !== null) set(result);
  }
  return (
    <div className="number-control">
      <button
        type="button"
        aria-label={`Disminuir ${label}`}
        disabled={disabled || next(-1) === null}
        onClick={() => move(-1)}
      >
        <Minus size={14} />
      </button>
      <input
        {...props}
        aria-label={props['aria-label'] ?? (props.id ? undefined : label)}
        disabled={disabled}
        type="text"
        inputMode="decimal"
        pattern={precision ? `-?[0-9]+([.,][0-9]{1,${precision}})?` : '-?[0-9]+'}
        value={current}
        onChange={(event) => set(event.target.value.replace(',', '.'))}
        onKeyDown={(event) => {
          if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
            event.preventDefault();
            move(event.key === 'ArrowUp' ? 1 : -1);
          }
          props.onKeyDown?.(event);
        }}
      />
      <button
        type="button"
        aria-label={`Aumentar ${label}`}
        disabled={disabled || next(1) === null}
        onClick={() => move(1)}
      >
        <Plus size={14} />
      </button>
    </div>
  );
}
