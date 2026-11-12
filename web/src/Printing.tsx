import { useEffect, useState } from 'react';
import { Printer, X } from 'lucide-react';
import { api, type Schema } from './api/client';

export const jobLabels: Record<Schema['PrintJob']['state'], string> = {
  pending: 'Pendiente',
  sending: 'Enviando',
  sent: 'Enviado a la impresora',
  error: 'Error de envío',
  uncertain: 'Resultado incierto',
  suspended: 'Suspendido',
};
export const jobKinds: Record<Schema['PrintJob']['kind'], string> = {
  sale: 'Ticket de venta',
  copy: 'Copia de ticket',
  test_print: 'Prueba de ticket',
  test_drawer: 'Prueba del cajón',
  drawer: 'Apertura manual',
  expense_drawer: 'Apertura por egreso',
};
export function usePrintJob(id: string) {
  const [job, setJob] = useState<Schema['PrintJob'] | null>(null),
    [error, setError] = useState('');
  useEffect(() => {
    if (!id) {
      setJob(null);
      return;
    }
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    setJob(null);
    setError('');
    async function poll() {
      try {
        const j = await api<Schema['PrintJob']>(
          `/pos/jobs/${id}`,
          'GET',
          undefined,
          {},
          controller.signal
        );
        if (controller.signal.aborted) return;
        setJob(j);
        setError('');
        if (j.state === 'pending' || j.state === 'sending') timer = setTimeout(poll, 700);
      } catch (e) {
        if (!controller.signal.aborted) {
          setError((e as Error).message);
          timer = setTimeout(poll, 3000);
        }
      }
    }
    void poll();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [id]);
  return { job, error };
}
export function PrintNotice({
  id,
  saleNumber,
  onView,
  onClose,
}: {
  id: string;
  saleNumber: number;
  onView: () => void;
  onClose: () => void;
}) {
  const { job, error } = usePrintJob(id);
  return (
    <div
      className={`print-notice ${job && ['error', 'uncertain', 'suspended'].includes(job.state) ? 'print-problem' : ''}`}
      role="status"
    >
      <Printer size={21} />
      <div>
        <strong>
          Venta #{String(saleNumber).padStart(5, '0')} ·{' '}
          {job ? jobLabels[job.state] : 'Consultando impresión…'}
        </strong>
        <small>
          {error ||
            job?.error ||
            (job?.state === 'sent'
              ? 'El envío no confirma la salida física del papel.'
              : 'La venta ya está registrada. Puedes continuar atendiendo.')}
        </small>
      </div>
      <button className="text-button" onClick={onView}>
        Ver comprobante
      </button>
      <button className="icon-button" aria-label="Cerrar estado de impresión" onClick={onClose}>
        <X size={16} />
      </button>
    </div>
  );
}
