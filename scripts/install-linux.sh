#!/usr/bin/env bash
set -euo pipefail
# Run from an extracted release: sudo ./install-linux.sh [listen] [certificate-hosts]
if [[ "$(id -u)" != "0" ]]; then
  echo "Ejecuta este instalador con sudo." >&2
  exit 1
fi
erp_package_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
erp_listen="${1:-127.0.0.1:8080}"
erp_hosts="${2:-localhost}"
erp_data_dir="/var/lib/raiz-erp/data"
if [[ ! -f "$erp_package_dir/erp" ]]; then
  echo "No se encontró el ejecutable erp junto al instalador." >&2
  exit 1
fi
if systemctl is-active --quiet raiz-erp; then
  echo "Detén el servicio antes de instalar una actualización: systemctl stop raiz-erp" >&2
  exit 1
fi
if ! id raiz-erp >/dev/null 2>&1; then
  useradd --system --user-group --home-dir /var/lib/raiz-erp --shell /usr/sbin/nologin raiz-erp
fi
install -d -m 0755 /opt/raiz-erp
install -d -m 0700 -o raiz-erp -g raiz-erp /var/lib/raiz-erp
install -m 0755 "$erp_package_dir/erp" /opt/raiz-erp/erp
if [[ ! -f "$erp_data_dir/config.json" ]]; then
  runuser -u raiz-erp -- /opt/raiz-erp/erp init --data-dir "$erp_data_dir" --listen "$erp_listen" --hosts "$erp_hosts"
fi
cat > /etc/systemd/system/raiz-erp.service <<'UNIT'
[Unit]
Description=Raíz ERP local
After=network.target

[Service]
Type=simple
User=raiz-erp
Group=raiz-erp
WorkingDirectory=/opt/raiz-erp
ExecStart=/opt/raiz-erp/erp serve --data-dir /var/lib/raiz-erp/data
Restart=on-failure
RestartSec=5
TimeoutStopSec=20
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/raiz-erp

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now raiz-erp
echo "Servicio instalado. Consulta el enlace de alta inicial con: journalctl -u raiz-erp"
