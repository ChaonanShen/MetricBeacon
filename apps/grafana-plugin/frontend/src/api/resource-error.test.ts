import { describe, expect, it } from 'vitest';

import { formatResourceError, isResourceNotFound } from './resource-error';

describe('resource errors', () => {
  it('recognizes missing resources from the contract code or HTTP status', () => {
    expect(isResourceNotFound({ data: { error: { code: 'resource_not_found' } } })).toBe(true);
    expect(isResourceNotFound({ status: 404 })).toBe(true);
    expect(isResourceNotFound({ status: 503, data: { error: { code: 'dependency_unavailable' } } })).toBe(false);
  });

  it('shows a structured boundary error without using arbitrary exception text', () => {
    expect(formatResourceError({ data: { error: { code: 'permission_denied', message: 'not allowed' } } })).toBe('permission_denied: not allowed');
    expect(formatResourceError({ status: 503, message: 'http://internal.service failed' })).toBe('request_failed: 请求失败（HTTP 503）');
    expect(formatResourceError(new Error('DEEPSEEK_API_KEY=test-key'))).toBe('dependency_unavailable: 请求失败，请检查服务状态后重试。');
  });
});
