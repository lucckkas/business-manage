import { useState } from 'react';
import { Field } from './ui';

export default function PasswordField({
  name,
  label,
  autoComplete = 'new-password',
}: {
  name: string;
  label: string;
  autoComplete?: string;
}) {
  const [value, setValue] = useState(''),
    [accepted, setAccepted] = useState(false);
  const weak = Array.from(value).length < 12;
  return (
    <div className="password-choice">
      <Field
        label={label}
        hint="Recomendamos al menos 12 caracteres. Máximo 128; puedes elegir una contraseña corta o vacía."
      >
        <input
          name={name}
          type="password"
          maxLength={128}
          autoComplete={autoComplete}
          value={value}
          onChange={(event) => {
            setValue(event.target.value);
            setAccepted(false);
          }}
        />
      </Field>
      {weak && (
        <div className="choice-warning">
          <p role="status">
            {value === ''
              ? 'Cualquier persona con acceso al sistema que conozca este usuario podrá entrar sin contraseña.'
              : 'Esta contraseña es corta y puede ser fácil de adivinar.'}
          </p>
          <label className="checkbox-field">
            <input
              name="allowWeakPassword"
              type="checkbox"
              checked={accepted}
              required
              onChange={(event) => setAccepted(event.target.checked)}
            />
            Usar esta contraseña aunque sea poco segura
          </label>
        </div>
      )}
    </div>
  );
}
