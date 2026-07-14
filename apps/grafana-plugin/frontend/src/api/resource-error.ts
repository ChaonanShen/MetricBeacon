type ResourceError = {
  status?: unknown;
  data?: {
    error?: {
      code?: unknown;
      message?: unknown;
    };
  };
};

function asResourceError(error: unknown): ResourceError | undefined {
  return error && typeof error === 'object' ? error as ResourceError : undefined;
}

export function isResourceNotFound(error: unknown): boolean {
  const resourceError = asResourceError(error);
  return resourceError?.status === 404 || resourceError?.data?.error?.code === 'resource_not_found';
}

export function formatResourceError(error: unknown): string {
  const resourceError = asResourceError(error);
  const code = resourceError?.data?.error?.code;
  const message = resourceError?.data?.error?.message;
  if (typeof code === 'string' && typeof message === 'string') {
    return `${code}: ${message}`;
  }
  if (typeof resourceError?.status === 'number') {
    return `request_failed: 请求失败（HTTP ${resourceError.status}）`;
  }
  return 'dependency_unavailable: 请求失败，请检查服务状态后重试。';
}
