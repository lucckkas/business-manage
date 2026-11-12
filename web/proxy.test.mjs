import assert from 'node:assert/strict';
import { createServer as createHTTPServer, request } from 'node:http';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { createServer, loadConfigFromFile } from 'vite';

function post(port, path, headers) {
  return new Promise((resolve, reject) => {
    const req = request(
      {
        hostname: '127.0.0.1',
        port,
        path,
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...headers },
      },
      (response) => {
        let body = '';
        response.setEncoding('utf8');
        response.on('data', (chunk) => {
          body += chunk;
        });
        response.on('error', reject);
        response.on('end', () => {
          try {
            assert.equal(response.statusCode, 200);
            resolve(JSON.parse(body));
          } catch (error) {
            reject(error);
          }
        });
      }
    );
    req.on('error', reject);
    req.end('{}');
  });
}

test('el proxy conserva Host y Origin para clientes independientes y no oculta orígenes ajenos', async () => {
  const backend = createHTTPServer((req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.end(
      JSON.stringify({
        host: req.headers.host,
        origin: req.headers.origin,
        csrf: req.headers['x-csrf-token'],
      })
    );
  });
  let vite, frontend;
  try {
    backend.listen(0, '127.0.0.1');
    await once(backend, 'listening');
    const target = `http://127.0.0.1:${backend.address().port}`;
    const root = fileURLToPath(new URL('.', import.meta.url));
    const loaded = await loadConfigFromFile(
      { command: 'serve', mode: 'development' },
      fileURLToPath(new URL('vite.config.ts', import.meta.url))
    );
    assert(loaded);
    const rule = loaded.config.server.proxy['/api'];
    vite = await createServer({
      ...loaded.config,
      configFile: false,
      root,
      logLevel: 'silent',
      server: {
        ...loaded.config.server,
        middlewareMode: true,
        hmr: false,
        proxy: { '/api': typeof rule === 'string' ? target : { ...rule, target } },
      },
    });
    frontend = createHTTPServer(vite.middlewares);
    frontend.listen(0, '127.0.0.1');
    await once(frontend, 'listening');
    const port = frontend.address().port;
    for (const host of [`127.0.0.1:${port}`, `localhost:${port}`]) {
      const origin = `http://${host}`;
      const echoed = await post(port, '/api/v1/auth/login', { Host: host, Origin: origin });
      assert.equal(echoed.host, host);
      assert.equal(echoed.origin, origin);
    }
    const echoed = await post(port, '/api/v1/products', {
      Origin: 'https://ajeno.example',
      'X-CSRF-Token': 'test-csrf',
    });
    assert.equal(echoed.host, `127.0.0.1:${port}`);
    assert.equal(echoed.origin, 'https://ajeno.example');
    assert.equal(echoed.csrf, 'test-csrf');
  } finally {
    if (vite) await vite.close();
    if (frontend) {
      frontend.close();
      frontend.closeAllConnections();
    }
    backend.close();
    backend.closeAllConnections();
  }
});
