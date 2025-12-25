import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

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
export const getUsers = (page = 1, perPage = 20) =>
  api.get(`/users?page=${page}&per_page=${perPage}`);

export const getUser = (id: number) => api.get(`/users/${id}`);

export const updateUser = (id: number, data: { name?: string; plan?: number }) =>
  api.patch(`/users/${id}`, data);

export const deleteUser = (id: number) => api.delete(`/users/${id}`);

export const approveUser = (id: number) => api.post(`/users/${id}/approve`);

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
