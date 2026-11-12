import { useState, type FormEvent } from 'react';
import { LogOut, LockKeyhole, UserRound, Building2 } from 'lucide-react';
import { api, type Schema, type Session } from './api/client';
import { ErrorMessage, Field } from './ui';
import { Heading } from './Pages';
import POSSettings from './POSSettings';
import PasswordField from './PasswordField';
export default function Settings({
  session,
  onSession,
  onSignedOut,
  onLogout,
  notify,
}: {
  session: Session;
  onSession: (s: Session) => void;
  onSignedOut: () => void;
  onLogout: () => void;
  notify: (s: string) => void;
}) {
  const [name, setName] = useState(session.user.name),
    [company, setCompany] = useState(session.company.name),
    [timezone, setTimezone] = useState(session.company.timezone),
    [error, setError] = useState(''),
    [busy, setBusy] = useState('');
  async function profile(event: FormEvent) {
    event.preventDefault();
    setBusy('profile');
    setError('');
    try {
      const user = await api<Schema['User']>('/auth/me', 'PATCH', { name });
      onSession({ ...session, user });
      notify('Perfil actualizado.');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  async function password(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    if (data.get('newPassword') !== data.get('repeatPassword')) {
      setError('Las contraseñas nuevas no coinciden.');
      return;
    }
    setBusy('password');
    setError('');
    try {
      await api('/auth/change-password', 'POST', {
        currentPassword: data.get('currentPassword'),
        newPassword: data.get('newPassword'),
        allowWeakPassword: data.get('allowWeakPassword') === 'on',
      });
      onSignedOut();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  async function saveCompany(event: FormEvent) {
    event.preventDefault();
    setBusy('company');
    setError('');
    try {
      const updated = await api<Schema['Company']>('/company', 'PATCH', {
        name: company,
        timezone,
      });
      onSession({ ...session, company: updated });
      notify('Datos de la empresa actualizados.');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  return (
    <>
      <Heading
        eyebrow="TU CUENTA Y TU ESPACIO"
        title="Ajustes"
        description="Administra tus datos y el acceso a tu espacio de trabajo."
      />
      <ErrorMessage message={error} />
      <div className="settings-forms">
        <section className="panel settings-form">
          <div className="panel-heading">
            <h2>
              <UserRound size={18} />
              Mi perfil
            </h2>
          </div>
          <form onSubmit={profile}>
            <Field label="Usuario">
              <input value={session.user.username} disabled />
            </Field>
            <Field label="Nombre visible">
              <input
                required
                minLength={2}
                maxLength={120}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <button className="button primary" disabled={!!busy}>
              {busy === 'profile' ? 'Guardando…' : 'Guardar perfil'}
            </button>
          </form>
          <div className="switch-user">
            <p>El carrito pendiente se conserva para tu usuario.</p>
            <button className="button secondary" disabled={!!busy} onClick={onLogout}>
              <LogOut size={16} />
              Cambiar de usuario
            </button>
          </div>
        </section>
        <section className="panel settings-form">
          <div className="panel-heading">
            <h2>
              <LockKeyhole size={18} />
              Cambiar contraseña
            </h2>
          </div>
          <form onSubmit={password}>
            <Field
              label="Contraseña actual"
              hint="Déjala vacía si actualmente entras sin contraseña."
            >
              <input
                name="currentPassword"
                type="password"
                maxLength={128}
                autoComplete="current-password"
              />
            </Field>
            <PasswordField name="newPassword" label="Nueva contraseña" />
            <Field label="Confirmar nueva contraseña">
              <input
                name="repeatPassword"
                type="password"
                maxLength={128}
                autoComplete="new-password"
              />
            </Field>
            <p className="form-note">
              Al cambiarla se cerrarán todas tus sesiones. Ingresa nuevamente con la contraseña
              nueva; si la dejaste vacía, introduce solo tu usuario.
            </p>
            <button className="button primary" disabled={!!busy}>
              {busy === 'password' ? 'Actualizando…' : 'Actualizar contraseña'}
            </button>
          </form>
        </section>
        {session.user.role === 'admin' && (
          <section className="panel settings-form">
            <div className="panel-heading">
              <h2>
                <Building2 size={18} />
                Datos de empresa
              </h2>
            </div>
            <form onSubmit={saveCompany}>
              <Field label="Nombre de empresa">
                <input
                  required
                  minLength={2}
                  maxLength={160}
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                />
              </Field>
              <Field label="Zona horaria">
                <input
                  required
                  value={timezone}
                  onChange={(e) => setTimezone(e.target.value)}
                  list="company-timezones"
                />
                <datalist id="company-timezones">
                  {[
                    'America/Santiago',
                    'America/Lima',
                    'America/Bogota',
                    'America/Mexico_City',
                    'Europe/Madrid',
                    'UTC',
                  ].map((t) => (
                    <option key={t} value={t} />
                  ))}
                </datalist>
              </Field>
              <Field
                label="Moneda"
                hint="Se conserva para mantener el significado de los importes históricos."
              >
                <input disabled value={session.company.currency} />
              </Field>
              <button className="button primary" disabled={!!busy}>
                {busy === 'company' ? 'Guardando…' : 'Guardar empresa'}
              </button>
            </form>
          </section>
        )}
      </div>
      {session.user.role === 'admin' && <POSSettings session={session} />}
    </>
  );
}
