import esbuild from 'rollup-plugin-esbuild';
import { nodeResolve } from '@rollup/plugin-node-resolve';

export default {
  input: 'src/module.tsx',
  external: (id) => id.startsWith('@grafana/') || id === 'react' || id === 'react-dom' || id === 'react/jsx-runtime',
  output: { file: '../dist/module.js', format: 'system', sourcemap: true, intro: 'const process = { env: { NODE_ENV: "production" } };' },
  plugins: [nodeResolve({ browser: true }), esbuild({ target: 'es2022', tsconfig: 'tsconfig.json' })],
};
