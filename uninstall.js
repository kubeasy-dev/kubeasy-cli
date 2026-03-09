#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
const pkg = require('./package.json');
const binFile = path.resolve(pkg.goBinary.path, pkg.goBinary.name);
if (fs.existsSync(binFile)) { fs.unlinkSync(binFile); console.log('kubeasy uninstalled'); }
