#!/usr/bin/env node
const https = require('https');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const os = require('os');

const pkg = require('./package.json');
const { name, path: binPath, url: urlTemplate } = pkg.goBinary;
const version = pkg.version;

const platformMap = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
const archMap = { x64: 'amd64', arm64: 'arm64' };

const platform = platformMap[process.platform];
const arch = archMap[process.arch];
if (!platform || !arch) {
  console.error(`Unsupported platform: ${process.platform}/${process.arch}`);
  process.exit(1);
}

const url = urlTemplate
  .replace(/\{\{version\}\}/g, version)
  .replace(/\{\{platform\}\}/g, platform)
  .replace(/\{\{arch\}\}/g, arch);

const dest = path.resolve(binPath);
fs.mkdirSync(dest, { recursive: true });

const tmpFile = path.join(os.tmpdir(), `kubeasy-${version}.tar.gz`);

console.log(`Downloading kubeasy v${version} for ${platform}/${arch}...`);

const MAX_REDIRECTS = 5;

function download(url, dest, cb, redirects) {
  if ((redirects || 0) > MAX_REDIRECTS) return cb(new Error('Too many redirects'));
  https.get(url, (res) => {
    if (res.statusCode >= 301 && res.statusCode <= 308 && res.statusCode !== 304) {
      const location = res.headers.location;
      res.resume();
      if (!location) return cb(new Error(`Redirect with no Location header (HTTP ${res.statusCode})`));
      return download(location, dest, cb, (redirects || 0) + 1);
    }
    if (res.statusCode !== 200) {
      res.resume();
      return cb(new Error(`HTTP ${res.statusCode}`));
    }
    const file = fs.createWriteStream(dest);
    res.pipe(file);
    file.on('error', cb);
    file.on('finish', () => file.close(cb));
  }).on('error', cb);
}

download(url, tmpFile, (err) => {
  if (err) {
    try { fs.unlinkSync(tmpFile); } catch (_) {}
    console.error('Download failed:', err.message);
    process.exit(1);
  }
  const result = spawnSync('tar', ['-xzf', tmpFile, '-C', dest, name], { stdio: 'inherit' });
  try { fs.unlinkSync(tmpFile); } catch (_) {}
  if (result.error || result.status !== 0) {
    console.error('Extraction failed:', result.error ? result.error.message : `exit code ${result.status}`);
    process.exit(1);
  }
  console.log(`kubeasy installed to ${path.join(dest, name)}`);
});
