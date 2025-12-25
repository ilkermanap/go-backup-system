// Go bindings for main.App
export interface Config {
  server_url: string;
  email: string;
  password: string;
  token: string;
  device_id: number;
  device_name: string;
  encryption_key: string;
  backup_dirs: string[];
  blacklist: string[];
  schedule_enabled: boolean;
  start_time: string;
  end_time: string;
  interval_minutes: number;
  skip_weekends: boolean;
  chunk_size: number;
}

export interface LoginResult {
  success: boolean;
  user: any;
  token: string;
}

export interface Device {
  id: number;
  name: string;
  created_at: string;
}

export interface BackupProgress {
  phase: string;
  current_file: string;
  total_files: number;
  done_files: number;
  total_bytes: number;
  done_bytes: number;
  percent: number;
}

export interface BackupStatus {
  is_running: boolean;
  last_backup: string;
  next_backup: string;
  files_count: number;
  total_size: number;
  device_id: number;
  device_name: string;
}

export interface BackupEntry {
  id: number;
  filename: string;
  size: number;
  created_at: string;
}

export interface QuotaInfo {
  total_gb: number;
  used_bytes: number;
  used_gb: number;
  percent: number;
}

export interface UsageInfo {
  device_count: number;
  backup_count: number;
  total_size: number;
  last_backup: string;
}

export function Login(email: string, password: string): Promise<LoginResult>;
export function Logout(): Promise<void>;
export function GetConfig(): Promise<Config>;
export function SaveConfig(config: Config): Promise<void>;
export function SelectDirectory(): Promise<string>;
export function AddBackupDirectory(dir: string): Promise<void>;
export function RemoveBackupDirectory(dir: string): Promise<void>;
export function GetDevices(): Promise<Device[]>;
export function RegisterDevice(name: string): Promise<Device>;
export function StartBackup(): Promise<void>;
export function StopBackup(): Promise<void>;
export function GetBackupStatus(): Promise<BackupStatus>;
export function GetBackupHistory(deviceId: number): Promise<BackupEntry[]>;
export function StartRestore(backupId: number, targetDir: string): Promise<void>;
export function GetQuota(): Promise<QuotaInfo>;
export function GetUsage(): Promise<UsageInfo>;
export function IsLoggedIn(): Promise<boolean>;
export function GetDataDir(): Promise<string>;
