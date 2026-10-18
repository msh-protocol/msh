declare function require(name: string): any;
const test = require('node:test');
const assert = require('node:assert');
import { exec } from './index';

test('msh TypeScript client executes command and returns structured response', async () => {
  const res = await exec('echo hello from typescript');
  assert.strictEqual(res.exit_code, 0);
  assert.strictEqual(res.status, 'success');
  assert.ok(res.stdout.includes('hello from typescript'));
  assert.strictEqual(res.truncated, false);
});

test('msh TypeScript client preserves failure exit codes', async () => {
  const res = await exec('exit 9');
  assert.strictEqual(res.exit_code, 9);
});
