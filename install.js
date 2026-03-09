#!/usr/bin/env node
const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
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

function download(url, dest, cb) {
  https.get(url, (res) => {
    if (res.statusCode === 301 || res.statusCode === 302) return download(res.headers.location, dest, cb);
    if (res.statusCode !== 200) return cb(new Error(`HTTP ${res.statusCode}`));
    const file = fs.createWriteStream(dest);
    res.pipe(file);
    file.on('finish', () => file.close(cb));
  }).on('error', cb);
}

download(url, tmpFile, (err) => {
  if (err) { console.error('Download failed:', err.message); process.exit(1); }
  try {
    execSync(`tar -xzf "${tmpFile}" -C "${dest}" "${name}"`, { stdio: 'inherit' });
    fs.unlinkSync(tmpFile);
    console.log(`kubeasy installed to ${path.join(dest, name)}`);
  } catch (e) {
    console.error('Extraction failed:', e.message);
    process.exit(1);
  }
});
