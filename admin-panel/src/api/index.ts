import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const api = axios.create({
  baseURL: API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Auth
export const login = (email: string, password: string) =>
  api.post('/auth/login', { email, password });

export const register = (name: string, email: string, password: string, plan: number) =>
  api.post('/auth/register', { name, email, password, plan });

export const getMe = () => api.get('/auth/me');

// Users
export interface UserFilters {
  page?: number;
  perPage?: number;
  search?: string;
  status?: 'approved' | 'pending' | 'active' | 'inactive';
  sort?: string;
  order?: 'asc' | 'desc';
}

export const getUsers = (filters: UserFilters = {}) => {
  const params = new URLSearchParams();
  if (filters.page) params.append('page', filters.page.toString());
  if (filters.perPage) params.append('per_page', filters.perPage.toString());
  if (filters.search) params.append('search', filters.search);
  if (filters.status) params.append('status', filters.status);
  if (filters.sort) params.append('sort', filters.sort);
  if (filters.order) params.append('order', filters.order);
  return api.get(`/users?${params.toString()}`);
};

export const getUser = (id: number) => api.get(`/users/${id}`);

export const updateUser = (id: number, data: { name?: string; plan?: number }) =>
  api.patch(`/users/${id}`, data);

export const deleteUser = (id: number) => api.delete(`/users/${id}`);

export const createUser = (data: { name: string; email: string; password: string; plan: number; role?: string }) =>
  api.post('/users', data);

export const approveUser = (id: number) => api.post(`/users/${id}/approve`);

export const resetPassword = (id: number, password: string) =>
  api.post(`/users/${id}/reset-password`, { password });

export const toggleUserStatus = (id: number) => api.post(`/users/${id}/toggle-status`);

export const bulkDeleteUsers = (ids: number[]) => api.post('/users/bulk-delete', { ids });

// Devices
export const getDevices = () => api.get('/devices');

export const createDevice = (name: string) => api.post('/devices', { name });

export const deleteDevice = (id: number) => api.delete(`/devices/${id}`);

// Backups
export const getBackups = (deviceId: number) => api.get(`/devices/${deviceId}/backups`);

export const deleteBackup = (deviceId: number, backupId: number) =>
  api.delete(`/devices/${deviceId}/backups/${backupId}`);

export const downloadBackup = (deviceId: number, backupId: number) =>
  `${API_URL}/devices/${deviceId}/backups/${backupId}/download`;

// Account
export const getQuota = () => api.get('/account/quota');

export const getUsage = () => api.get('/account/usage');

export default api;
