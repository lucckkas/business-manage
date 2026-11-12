# Instalación y acceso local

## Paquete portable

Verifica el SHA-256 del archivo contra `SHA256SUMS`, descomprímelo y ejecuta el binario desde la carpeta extraída. En Linux:

```bash
./erp serve --data-dir ./data
```

En PowerShell:

```powershell
.\erp.exe serve --data-dir .\data
```

El primer arranque crea configuración, certificados y token de instalación. La consola muestra el enlace de alta inicial. Abre ese enlace en el mismo equipo y guarda las credenciales del administrador. Al cerrar la consola se detiene el modo portable; para inicio automático utiliza un servicio.

Datos predeterminados: `data/`; respaldos: `data-backups/`, carpeta hermana. El bloqueo de proceso es `data.lock`, fuera del directorio intercambiable. Utiliza rutas de disco local para la base; los clientes acceden por HTTP/HTTPS y nunca abren el archivo SQLite.

## Compartir por red con HTTPS

Para una instalación nueva, elige una IP fija o un nombre local resoluble desde los clientes. Ejemplo:

```bash
./erp init --data-dir ./data --listen 0.0.0.0:8443 --hosts erp.local,192.168.1.20
./erp serve --data-dir ./data
```

Escuchar fuera de loopback activa TLS automáticamente. Los certificados incluyen los nombres e IP indicados, el nombre del equipo, localhost y las direcciones de loopback. El servidor crea una autoridad local exclusiva de esta instalación.

1. Distribuye **únicamente** `data/certificates/ca.crt` a los equipos clientes e instálalo en el almacén de autoridades de confianza de sus sistemas o navegadores. La clave `ca.key` permanece en el servidor y los respaldos.
2. Permite el puerto configurado en el firewall para la red privada de la empresa.
3. Completa el alta desde `https://localhost:8443` en el servidor. Si Linux no tiene navegador, utiliza un túnel SSH local y confía en la misma autoridad en tu equipo.
4. En los clientes abre `https://erp.local:8443` o la IP incluida en el certificado.

Para una instalación existente detén el servicio y edita `config.json`: cambia `listen` y activa `tls`. Incorpora el nombre o IP mediante `./erp certificates --data-dir ./data --hosts erp.local,192.168.1.20` antes de reiniciar. HTTP se admite únicamente al escuchar en direcciones IP de loopback.

No se descargan fuentes, iconos, scripts ni estilos durante el uso. Todo forma parte del ejecutable. Los equipos cliente necesitan un navegador moderno compatible con ES2022.

## Servicio Linux

Desde un paquete extraído:

```bash
sudo bash ./install-linux.sh 0.0.0.0:8443 erp.local,192.168.1.20
sudo journalctl -u raiz-erp
```

Instala el ejecutable en `/opt/raiz-erp`, crea el usuario restringido `raiz-erp` y guarda datos en `/var/lib/raiz-erp/data`. Los respaldos quedan en `/var/lib/raiz-erp/data-backups`. El servicio utiliza `systemd` y se reinicia ante fallos. Requiere una distribución con `systemd`, `useradd` y `runuser`.

```bash
sudo systemctl stop raiz-erp
sudo systemctl start raiz-erp
sudo systemctl status raiz-erp
```

## Servicio Windows

Abre PowerShell como administrador en el paquete extraído:

```powershell
.\install-windows.ps1 -Listen '0.0.0.0:8443' -Hosts 'erp.local,192.168.1.20'
```

Instala en `%ProgramFiles%\RaizERP`, guarda datos en `%ProgramData%\RaizERP\data` y registra el servicio nativo `RaizERP` como LocalService. Restringe el directorio de estado a administradores, SYSTEM y LocalService. El script necesita permiso para registrar el servicio y establecer sus ACL.

```powershell
Stop-Service RaizERP
Start-Service RaizERP
Get-Service RaizERP
```

El código de instalación se encuentra en `%ProgramData%\RaizERP\data\setup.token` hasta completar el alta. No hay credenciales predeterminadas. En modo portable, protege la carpeta de datos con los permisos del usuario que ejecuta el servidor.
