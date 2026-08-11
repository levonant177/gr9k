const BASE = import.meta.env.PUBLIC_API_URL || '/api/v1';

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers
    },
    ...options
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || 'Request failed');
  }

  return res.json();
}

export const api = {
  items: {
    list: () => request<any[]>('/items'),
    get: (id: string) => request<any>(`/items/${id}`),
    create: (data: any) => request<{ id: string }>('/items', {
      method: 'POST',
      body: JSON.stringify(data)
    })
  },
  bom: {
    get: (itemId: string) => request<any>(`/bom/${itemId}`),
    requirements: (itemId: string, qty = 1) =>
      request<any[]>(`/bom/${itemId}/requirements?qty=${qty}`),
    create: (data: any) => request<{ id: string }>('/bom', {
      method: 'POST',
      body: JSON.stringify(data)
    })
  },
  availability: {
    list: () => request<any[]>('/availability'),
    get: (article: string) => request<any>(`/availability/${article}`)
  },
  warehouse: {
    zones: () => request<any[]>('/warehouse/zones'),
    locations: (zone?: string) =>
      request<any[]>(`/warehouse/locations${zone ? '?zone=' + zone : ''}`),
    createWave: (data: any) =>
      request<any>('/warehouse/waves', { method: 'POST', body: JSON.stringify(data) }),
    getWave: (id: string) => request<any>(`/warehouse/waves/${id}`),
    releaseWave: (id: string) =>
      request<any>(`/warehouse/waves/${id}/release`, { method: 'POST' }),
    startInventory: (data: any) =>
      request<any>('/warehouse/inventory', { method: 'POST', body: JSON.stringify(data) }),
    confirmCount: (id: string, data: any) =>
      request<any>(`/warehouse/inventory/${id}/confirm`, {
        method: 'POST',
        body: JSON.stringify(data)
      }),
    completeInventory: (id: string) =>
      request<any>(`/warehouse/inventory/${id}/complete`, { method: 'POST' })
  }
};
