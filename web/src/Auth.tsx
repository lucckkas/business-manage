import { useState, type FormEvent } from 'react';
import { ArrowRight, Check, ShieldCheck, Sprout } from 'lucide-react';
import { api, type Schema, type Session } from './api/client';
import { ErrorMessage, Field } from './ui';
import PasswordField from './PasswordField';

export default function Auth({
  setup,
  local,
  token,
  onSetup,
  onLogin,
}: {
  setup: boolean;
  local: boolean;
  token: string;
  onSetup: () => void;
  onLogin: (s: Session) => void;
}) {
  const [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setBusy(true);
    const values = new FormData(event.currentTarget);
    try {
      if (setup) {
        const body: Schema['SetupRequest'] = {
          company: {
            name: String(values.get('company')),
            currency: String(values.get('currency')) as Schema['Company']['currency'],
            timezone: String(values.get('timezone')),
          },
          name: String(values.get('name')),
          username: String(values.get('username')),
          password: String(values.get('password')),
          allowWeakPassword: values.get('allowWeakPassword') === 'on',
        };
        await api('/setup', 'POST', body, { Authorization: `Setup ${values.get('token')}` });
        onSetup();
      } else {
        const session = await api<Session>('/auth/login', 'POST', {
          username: values.get('username'),
          password: values.get('password'),
        });
        onLogin(session);
      }
    } catch (error) {
      setError((error as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-layout">
      <aside className="auth-story">
        <a className="brand" href="#/">
          <span className="brand-symbol">
            <Sprout size={24} />
          </span>
          <span>
            raíz<span className="brand-dot">.</span>
          </span>
        </a>
        <div>
          <span className="eyebrow light">GESTIÓN CON SENTIDO</span>
          <h1>
            Más claridad.
            <br />
            Más tiempo para
            <br />
            tu negocio.
          </h1>
          <p>
            Productos, ventas e inventario.
            <br />
            Todo conectado, cerca de ti.
          </p>
          <div className="story-lines">
            <span />
            <span />
            <span />
          </div>
        </div>
        <span className="local-note">
          <ShieldCheck size={17} /> Tus datos permanecen en tu empresa
        </span>
      </aside>
      <main className="auth-content">
        <div className="auth-form">
          <span className="eyebrow">{setup ? 'TU EMPRESA EMPIEZA AQUÍ' : 'BIENVENIDO A RAÍZ'}</span>
          <h2>{setup ? 'Hagamos espacio para crecer.' : 'Un nuevo día, todo en orden.'}</h2>
          <p className="muted">
            {setup
              ? 'Configura tu espacio de trabajo y crea el primer acceso.'
              : 'Ingresa a tu espacio de gestión comercial.'}
          </p>
          {setup && !local ? (
            <ErrorMessage message="Completa la configuración inicial desde un navegador en el equipo servidor. Después podrás acceder desde la red local." />
          ) : (
            <form onSubmit={submit}>
              <ErrorMessage message={error} />
              {setup && (
                <>
                  <div className="form-divider">
                    <span>01</span> Tu empresa
                  </div>
                  <Field label="Nombre de la empresa">
                    <input
                      name="company"
                      required
                      minLength={2}
                      maxLength={160}
                      autoComplete="organization"
                      autoFocus
                      placeholder="El nombre de tu negocio"
                    />
                  </Field>
                  <div className="form-grid">
                    <Field label="Moneda">
                      <select name="currency" defaultValue="CLP">
                        {['CLP', 'USD', 'EUR', 'PEN', 'MXN', 'ARS', 'COP', 'UYU', 'BOB', 'BRL'].map(
                          (c) => (
                            <option key={c}>{c}</option>
                          )
                        )}
                      </select>
                    </Field>
                    <Field label="Zona horaria">
                      <input
                        name="timezone"
                        required
                        defaultValue={
                          Intl.DateTimeFormat().resolvedOptions().timeZone || 'America/Santiago'
                        }
                        list="timezones"
                      />
                      <datalist id="timezones">
                        {[
                          'America/Santiago',
                          'America/Lima',
                          'America/Mexico_City',
                          'America/Argentina/Buenos_Aires',
                          'America/Bogota',
                          'Europe/Madrid',
                          'UTC',
                        ].map((t) => (
                          <option key={t} value={t} />
                        ))}
                      </datalist>
                    </Field>
                  </div>
                  <div className="form-divider">
                    <span>02</span> Tu acceso de administrador
                  </div>
                  <Field label="Tu nombre">
                    <input name="name" required minLength={2} maxLength={120} autoComplete="name" />
                  </Field>
                </>
              )}
              <Field label="Usuario">
                <input
                  name="username"
                  required
                  minLength={setup ? 3 : 1}
                  maxLength={64}
                  autoComplete="username"
                  autoCapitalize="none"
                  autoFocus={!setup}
                  placeholder="Tu usuario"
                />
              </Field>
              {setup ? (
                <PasswordField name="password" label="Contraseña" />
              ) : (
                <Field
                  label="Contraseña"
                  hint="Si tu cuenta no tiene contraseña, deja este campo vacío."
                >
                  <input
                    name="password"
                    type="password"
                    maxLength={128}
                    autoComplete="current-password"
                    placeholder="Tu contraseña"
                  />
                </Field>
              )}
              {setup && (
                <Field
                  label="Código de instalación"
                  hint="Encontrarás el código en la ventana del servidor al iniciar el sistema."
                >
                  <input
                    name="token"
                    type="password"
                    required
                    defaultValue={token}
                    autoComplete="off"
                  />
                </Field>
              )}
              <button className="button primary full" disabled={busy}>
                {busy ? 'Un momento…' : setup ? 'Crear mi espacio' : 'Iniciar sesión'}
                {!busy && (setup ? <Check size={17} /> : <ArrowRight size={17} />)}
              </button>
              {!setup && (
                <p className="auth-help">
                  ¿Necesitas acceso? Contacta al administrador de tu empresa.
                </p>
              )}
            </form>
          )}
          <div className="auth-footer">
            <span className="status-dot" /> Gestión local · Sin depender de Internet
          </div>
        </div>
      </main>
    </div>
  );
}
