import { useState, useEffect, useMemo } from 'react';
import {
  Layout, Menu, Card, Button, Progress, Statistic, Row, Col,
  List, Tag, Space, Typography, message, Modal, Input, Empty, Alert, Form,
  Spin
} from 'antd';
import {
  CloudUploadOutlined, CloudDownloadOutlined, SettingOutlined,
  FolderOutlined, LogoutOutlined, PlayCircleOutlined,
  PauseCircleOutlined, PlusOutlined, DeleteOutlined,
  ReloadOutlined, HddOutlined, LockOutlined, KeyOutlined,
  FileOutlined, DownloadOutlined, CalendarOutlined,
  ZoomInOutlined, ZoomOutOutlined, ArrowLeftOutlined, HomeOutlined
} from '@ant-design/icons';

const { Header, Sider, Content } = Layout;
const { Title, Text } = Typography;

interface DashboardProps {
  onLogout: () => void;
}

// Types for Time Machine UI
interface CatalogFile {
  orig_path: string;
  directory: string;
  file_name: string;
  latest_version: string;
  version_count: number;
  size: number;
}

interface FileVersion {
  timestamp: string;
  size: number;
  hash: string;
}

// Mock functions for development
const mockFunctions = {
  GetBackupStatus: async () => ({
    is_running: false,
    last_backup: '2024-01-15 14:30',
    files_count: 1234,
    total_size: 5368709120,
    device_name: 'My Computer'
  }),
  GetQuota: async () => ({
    total_gb: 10,
    used_gb: 3.5,
    percent: 35
  }),
  GetConfig: async () => ({
    backup_dirs: ['/home/user/Documents', '/home/user/Photos'],
    server_url: 'http://localhost:8080',
    device_name: 'My Computer',
    encryption_key: ''
  }),
  SaveConfig: async (_cfg: any) => {},
  SetEncryptionKey: async (_key: string) => {},
  SelectDirectory: async () => '/home/user/NewFolder',
  AddBackupDirectory: async () => {},
  RemoveBackupDirectory: async () => {},
  RegisterDevice: async (name: string) => ({ id: 1, name }),
  StartBackup: async () => {},
  StopBackup: async () => {},
  Logout: async () => { localStorage.removeItem('token'); },
  GetBackupDates: async () => ['2024-01-15', '2024-01-14', '2024-01-13'],
  GetCatalogFiles: async () => [] as CatalogFile[],
  GetCatalogFilesAtDate: async (_date: string) => [] as CatalogFile[],
  GetFileHistory: async (_path: string) => [] as FileVersion[],
  RestoreToDate: async (_date: string, _targetDir: string) => {},
  RestoreFile: async (_origPath: string, _dateStr: string, _targetDir: string) => {},
  RestoreDirectory: async (_dirPath: string, _dateStr: string, _targetDir: string) => {},
  HasLocalCatalog: async () => true,
  RecoverCatalog: async () => {},
};

// Use Wails or mock
const go = (window as any).go?.main?.App || mockFunctions;

export default function Dashboard({ onLogout }: DashboardProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [activeTab, setActiveTab] = useState('backup');
  const [status, setStatus] = useState<any>(null);
  const [quota, setQuota] = useState<any>(null);
  const [config, setConfig] = useState<any>(null);
  const [isRunning, setIsRunning] = useState(false);
  const [progress, setProgress] = useState(0);
  const [encryptionKeyModal, setEncryptionKeyModal] = useState(false);
  const [encryptionKey, setEncryptionKey] = useState('');
  const [encryptionKeyConfirm, setEncryptionKeyConfirm] = useState('');
  const [deviceModal, setDeviceModal] = useState(false);
  const [deviceName, setDeviceName] = useState('');
  const [progressMessage, setProgressMessage] = useState('');

  // Time Machine state
  const [backupDates, setBackupDates] = useState<string[]>([]);
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [catalogFiles, setCatalogFiles] = useState<CatalogFile[]>([]);
  const [selectedFile, setSelectedFile] = useState<CatalogFile | null>(null);
  const [restoreLoading, setRestoreLoading] = useState(false);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [hasLocalCatalog, setHasLocalCatalog] = useState(true);
  const [catalogRecovering, setCatalogRecovering] = useState(false);
  const [timelineZoom, setTimelineZoom] = useState(10); // Show max 10 items initially
  const [currentPath, setCurrentPath] = useState<string>('/'); // File browser current path

  useEffect(() => {
    loadData();

    // Listen for backup progress events
    const events = (window as any).runtime;
    console.log('Setting up event listeners, runtime:', events ? 'available' : 'not available');
    if (events?.EventsOn) {
      events.EventsOn('backup:progress', (p: any) => {
        console.log('Backup progress event received:', p);

        // Ensure isRunning is true when we receive progress
        if (p.phase !== 'complete') {
          setIsRunning(true);
        }

        setProgress(p.percent || 0);

        // Always update message if present
        if (p.message) {
          setProgressMessage(p.message);
        }

        if (p.phase === 'complete') {
          setIsRunning(false);
          setProgressMessage('');
          if (p.total_files === 0) {
            message.info(p.message || 'Yedeklenecek yeni dosya bulunamadı');
          } else {
            message.success(p.message || `Yedekleme tamamlandı! (${p.done_files} dosya)`);
            // Update hasLocalCatalog since we just added files
            setHasLocalCatalog(true);
          }
          loadData();
        }
      });

      events.EventsOn('backup:error', (err: string) => {
        console.log('Backup error event received:', err);
        setIsRunning(false);
        setProgressMessage('');
        message.error(`Hata: ${err}`);
      });

      // Catalog recovery events
      events.EventsOn('catalog:recovering', () => {
        setCatalogRecovering(true);
        message.info('Katalog sunucudan indiriliyor...');
      });

      events.EventsOn('catalog:recovered', () => {
        setCatalogRecovering(false);
        setHasLocalCatalog(true);
        message.success('Katalog başarıyla kurtarıldı!');
        loadTimeMachineData();
      });

      events.EventsOn('catalog:error', (err: string) => {
        setCatalogRecovering(false);
        message.error(`Katalog kurtarma hatası: ${err}`);
      });

      // Restore events
      events.EventsOn('restore:progress', (p: any) => {
        console.log('Restore progress:', p);
        if (p.message) {
          message.info(p.message);
        }
      });

      events.EventsOn('restore:error', (err: string) => {
        console.error('Restore error:', err);
        setRestoreLoading(false);
        message.error(`Geri yükleme hatası: ${err}`);
      });

      events.EventsOn('restore:complete', () => {
        console.log('Restore complete');
        setRestoreLoading(false);
        message.success('Dosya başarıyla geri yüklendi!');
      });
    }

    // Check if local catalog exists
    checkLocalCatalog();
  }, []);

  const loadData = async () => {
    try {
      const [s, q, c] = await Promise.all([
        go.GetBackupStatus(),
        go.GetQuota(),
        go.GetConfig()
      ]);
      setStatus(s);
      setQuota(q);
      setConfig(c);
      setIsRunning(s?.is_running || false);
    } catch (err) {
      console.error('Failed to load data:', err);
    }
  };

  const checkLocalCatalog = async () => {
    try {
      const has = await go.HasLocalCatalog();
      setHasLocalCatalog(has);
    } catch (err) {
      console.error('Failed to check local catalog:', err);
    }
  };

  const handleRecoverCatalog = async () => {
    if (!config?.encryption_key) {
      message.warning('Önce şifreleme anahtarı girilmeli');
      setEncryptionKeyModal(true);
      return;
    }
    if (!config?.device_id) {
      message.warning('Önce cihaz kaydı yapılmalı');
      setDeviceModal(true);
      return;
    }

    try {
      await go.RecoverCatalog();
    } catch (err: any) {
      message.error(err.message || 'Katalog kurtarma başlatılamadı');
    }
  };

  const handleStartBackup = async () => {
    // Check if encryption key is set
    if (!config?.encryption_key) {
      setEncryptionKeyModal(true);
      return;
    }

    // Check if device is registered
    if (!config?.device_id || config.device_id === 0) {
      message.warning('Önce cihaz kaydı yapılmalı');
      setDeviceModal(true);
      return;
    }

    // Check if there are backup directories
    if (!config?.backup_dirs || config.backup_dirs.length === 0) {
      message.warning('Önce yedeklenecek dizin ekleyin');
      return;
    }

    try {
      setIsRunning(true);
      setProgress(0);
      setProgressMessage('Yedekleme başlatılıyor...');
      await go.StartBackup();
    } catch (err: any) {
      setIsRunning(false);
      message.error(err.message || 'Yedekleme başlatılamadı');
    }
  };

  const handleSaveEncryptionKey = async () => {
    if (!encryptionKey) {
      message.error('Şifreleme anahtarı giriniz');
      return;
    }
    if (encryptionKey.length < 8) {
      message.error('Şifreleme anahtarı en az 8 karakter olmalı');
      return;
    }
    if (encryptionKey !== encryptionKeyConfirm) {
      message.error('Şifreleme anahtarları eşleşmiyor');
      return;
    }

    try {
      await go.SetEncryptionKey(encryptionKey);
      setConfig({ ...config, encryption_key: encryptionKey });
      setEncryptionKeyModal(false);
      setEncryptionKey('');
      setEncryptionKeyConfirm('');
      message.success('Şifreleme anahtarı kaydedildi');
    } catch (err: any) {
      message.error(err.message || 'Anahtar kaydedilemedi');
    }
  };

  const handleRegisterDevice = async () => {
    if (!deviceName || deviceName.length < 2) {
      message.error('Cihaz adı en az 2 karakter olmalı');
      return;
    }

    try {
      const device = await go.RegisterDevice(deviceName);
      message.success(`Cihaz kaydedildi: ${device.name}`);
      setDeviceModal(false);
      setDeviceName('');
      loadData();
    } catch (err: any) {
      message.error(err.message || 'Cihaz kaydedilemedi');
    }
  };

  const handleStopBackup = async () => {
    try {
      await go.StopBackup();
      setIsRunning(false);
      setProgressMessage('');
      message.info('Yedekleme durduruldu');
    } catch (err: any) {
      message.error(err.message);
    }
  };

  const handleAddDirectory = async () => {
    try {
      const dir = await go.SelectDirectory();
      if (dir) {
        await go.AddBackupDirectory(dir);
        message.success('Dizin eklendi');
        loadData();
      }
    } catch (err: any) {
      message.error(err.message);
    }
  };

  const handleRemoveDirectory = async (dir: string) => {
    Modal.confirm({
      title: 'Dizini Kaldır',
      content: `"${dir}" dizinini yedekleme listesinden kaldırmak istediğinize emin misiniz?`,
      okText: 'Kaldır',
      cancelText: 'İptal',
      okType: 'danger',
      onOk: async () => {
        try {
          await go.RemoveBackupDirectory(dir);
          message.success('Dizin kaldırıldı');
          loadData();
        } catch (err: any) {
          message.error(err.message);
        }
      }
    });
  };

  const handleLogout = async () => {
    try {
      await go.Logout();
      onLogout();
    } catch (err) {
      onLogout();
    }
  };

  // Time Machine functions
  const loadTimeMachineData = async () => {
    console.log('loadTimeMachineData called');
    setCatalogLoading(true);
    try {
      console.log('Fetching backup dates...');
      let dates: string[] = [];
      let files: CatalogFile[] = [];

      try {
        dates = await go.GetBackupDates() || [];
        console.log('Backup dates received:', dates);
      } catch (e) {
        console.error('Error fetching dates:', e);
      }

      try {
        files = await go.GetCatalogFiles() || [];
        console.log('Catalog files received:', files);
      } catch (e) {
        console.error('Error fetching files:', e);
      }

      setBackupDates(dates);
      setCatalogFiles(files);

      // Update hasLocalCatalog based on actual data
      if (dates.length > 0 || files.length > 0) {
        setHasLocalCatalog(true);
      }

      if (dates.length > 0 && !selectedDate) {
        setSelectedDate(dates[0]);
      }

      if (dates.length === 0 && files.length === 0) {
        console.log('No backup data found in local catalog');
      }
    } catch (err: any) {
      console.error('Failed to load Time Machine data:', err);
      message.error(`Katalog yüklenemedi: ${err.message || err}`);
    } finally {
      console.log('loadTimeMachineData finished, setting catalogLoading to false');
      setCatalogLoading(false);
    }
  };

  const handleSelectFile = (file: CatalogFile) => {
    setSelectedFile(file);
  };

  // Restore a specific file version
  const handleRestoreFileVersion = async (file: CatalogFile, version: FileVersion) => {
    // Select target directory
    try {
      const targetDir = await go.SelectDirectory();
      if (!targetDir) return;

      setRestoreLoading(true);
      await go.RestoreFile(file.orig_path, version.timestamp, targetDir);
      message.success(`${file.file_name} geri yükleniyor...`);
    } catch (err: any) {
      message.error(err.message || 'Dosya geri yüklenemedi');
    } finally {
      setRestoreLoading(false);
    }
  };

  // Load Time Machine data when switching to restore tab
  useEffect(() => {
    if (activeTab === 'restore') {
      loadTimeMachineData();
    }
  }, [activeTab]);

  // Load files for selected date
  useEffect(() => {
    const loadFilesForDate = async () => {
      if (!selectedDate) return;

      setCatalogLoading(true);
      try {
        console.log('Loading files for date:', selectedDate);
        const files = await go.GetCatalogFilesAtDate(selectedDate);
        console.log('Files loaded for date:', files?.length || 0);
        setCatalogFiles(files || []);
        // Keep current path - don't reset to root
      } catch (err) {
        console.error('Failed to load files for date:', err);
      } finally {
        setCatalogLoading(false);
      }
    };

    loadFilesForDate();
  }, [selectedDate]);

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  // File browser: compute current directory contents
  interface BrowserItem {
    name: string;
    isDir: boolean;
    path: string;
    size?: number;
    fileCount?: number; // for directories
    file?: CatalogFile; // original file for restore
  }

  const browserItems = useMemo((): BrowserItem[] => {
    if (catalogFiles.length === 0) return [];

    const items: BrowserItem[] = [];
    const seenDirs = new Set<string>();

    catalogFiles.forEach(file => {
      // Check if file is under current path
      if (!file.orig_path.startsWith(currentPath)) return;

      // Get relative path from current directory
      const relativePath = file.orig_path.slice(currentPath.length);
      const parts = relativePath.split('/').filter(p => p);

      if (parts.length === 0) return;

      if (parts.length === 1) {
        // Direct file in current directory
        items.push({
          name: parts[0],
          isDir: false,
          path: file.orig_path,
          size: file.size,
          file: file
        });
      } else {
        // Subdirectory
        const dirName = parts[0];
        const dirPath = currentPath + dirName + '/';
        if (!seenDirs.has(dirPath)) {
          seenDirs.add(dirPath);
          // Count files in this directory
          const filesInDir = catalogFiles.filter(f => f.orig_path.startsWith(dirPath));
          const totalSize = filesInDir.reduce((sum, f) => sum + f.size, 0);
          items.push({
            name: dirName,
            isDir: true,
            path: dirPath,
            size: totalSize,
            fileCount: filesInDir.length
          });
        }
      }
    });

    // Sort: directories first, then files
    return items.sort((a, b) => {
      if (a.isDir && !b.isDir) return -1;
      if (!a.isDir && b.isDir) return 1;
      return a.name.localeCompare(b.name);
    });
  }, [catalogFiles, currentPath]);

  // Navigate to directory
  const navigateToDir = (path: string) => {
    setCurrentPath(path);
    setSelectedFile(null);
  };

  // Go up one directory
  const navigateUp = () => {
    if (currentPath === '/') return;
    const parts = currentPath.slice(0, -1).split('/');
    parts.pop();
    const newPath = parts.length === 0 ? '/' : parts.join('/') + '/';
    setCurrentPath(newPath);
    setSelectedFile(null);
  };

  // Restore directory with all its files
  const handleRestoreDirectory = async (dirPath: string) => {
    console.log('[handleRestoreDirectory] dirPath:', dirPath, 'selectedDate:', selectedDate);
    if (!selectedDate) {
      message.warning('Lütfen bir tarih seçin');
      return;
    }

    try {
      const targetDir = await go.SelectDirectory();
      if (!targetDir) return;

      setRestoreLoading(true);
      // Use RestoreDirectory API - it handles getting all files at selected date
      await go.RestoreDirectory(dirPath, selectedDate, targetDir);
      message.info('Dizin geri yükleme başlatıldı...');
    } catch (err: any) {
      message.error(err.message || 'Geri yükleme başarısız');
      setRestoreLoading(false);
    }
    // Note: setRestoreLoading(false) will be called by restore:complete or restore:error event
  };

  // Breadcrumb parts
  const breadcrumbParts = useMemo(() => {
    if (currentPath === '/') return [{ name: 'Kök', path: '/' }];
    const parts = currentPath.split('/').filter(p => p);
    const result = [{ name: 'Kök', path: '/' }];
    let accumulated = '/';
    parts.forEach(p => {
      accumulated += p + '/';
      result.push({ name: p, path: accumulated });
    });
    return result;
  }, [currentPath]);

  const menuItems = [
    { key: 'backup', icon: <CloudUploadOutlined />, label: 'Yedekleme' },
    { key: 'restore', icon: <CloudDownloadOutlined />, label: 'Geri Yükleme' },
    { key: 'settings', icon: <SettingOutlined />, label: 'Ayarlar' },
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        theme="dark"
      >
        <div style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'white',
          fontSize: collapsed ? 16 : 18,
          fontWeight: 'bold'
        }}>
          {collapsed ? 'BC' : 'Backup Client'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[activeTab]}
          items={menuItems}
          onClick={({ key }) => setActiveTab(key)}
        />
        <div style={{ position: 'absolute', bottom: 60, width: '100%', padding: 16 }}>
          <Button
            type="text"
            icon={<LogoutOutlined />}
            onClick={handleLogout}
            style={{ color: 'rgba(255,255,255,0.65)', width: '100%' }}
          >
            {!collapsed && 'Çıkış'}
          </Button>
        </div>
      </Sider>

      <Layout>
        <Header style={{
          background: '#fff',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <Title level={4} style={{ margin: 0 }}>
            {activeTab === 'backup' && 'Yedekleme'}
            {activeTab === 'restore' && 'Geri Yükleme'}
            {activeTab === 'settings' && 'Ayarlar'}
          </Title>
          <Space>
            <Tag
              icon={<HddOutlined />}
              color={config?.device_id ? "blue" : "orange"}
              style={{ cursor: config?.device_id ? 'default' : 'pointer' }}
              onClick={() => !config?.device_id && setDeviceModal(true)}
            >
              {config?.device_id ? (status?.device_name || config?.device_name || 'Cihaz') : 'Cihaz Kayıtsız (tıkla)'}
            </Tag>
            <Button icon={<ReloadOutlined />} onClick={loadData}>
              Yenile
            </Button>
          </Space>
        </Header>

        <Content style={{ margin: 24 }}>
          {activeTab === 'backup' && (
            <Row gutter={[24, 24]}>
              {/* Status Card */}
              <Col span={24}>
                <Card>
                  <Row gutter={24} align="middle">
                    <Col span={6}>
                      <Statistic
                        title="Yedeklenen Dosya"
                        value={status?.files_count || 0}
                        suffix="dosya"
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic
                        title="Toplam Boyut"
                        value={formatBytes(status?.total_size || 0)}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic
                        title="Son Yedekleme"
                        value={status?.last_backup || '-'}
                      />
                    </Col>
                    <Col span={6}>
                      <Space>
                        {isRunning ? (
                          <Button
                            type="primary"
                            danger
                            icon={<PauseCircleOutlined />}
                            onClick={handleStopBackup}
                            size="large"
                          >
                            Durdur
                          </Button>
                        ) : (
                          <Button
                            type="primary"
                            icon={<PlayCircleOutlined />}
                            onClick={handleStartBackup}
                            size="large"
                          >
                            Yedekle
                          </Button>
                        )}
                      </Space>
                    </Col>
                  </Row>
                  {isRunning && (
                    <div style={{ marginTop: 16 }}>
                      <Progress
                        percent={Math.round(progress)}
                        status="active"
                      />
                      {progressMessage && (
                        <Text type="secondary" style={{ display: 'block', marginTop: 8, textAlign: 'center' }}>
                          {progressMessage}
                        </Text>
                      )}
                    </div>
                  )}
                </Card>
              </Col>

              {/* Quota Card */}
              <Col span={8}>
                <Card title="Kota Kullanımı">
                  <Progress
                    type="dashboard"
                    percent={quota?.percent || 0}
                    format={percent => `${percent}%`}
                  />
                  <div style={{ textAlign: 'center', marginTop: 16 }}>
                    <Text type="secondary">
                      {quota?.used_gb?.toFixed(2) || 0} GB / {quota?.total_gb || 0} GB
                    </Text>
                  </div>
                </Card>
              </Col>

              {/* Directories Card */}
              <Col span={16}>
                <Card
                  title="Yedekleme Dizinleri"
                  extra={
                    <Button
                      type="primary"
                      icon={<PlusOutlined />}
                      onClick={handleAddDirectory}
                    >
                      Dizin Ekle
                    </Button>
                  }
                >
                  {config?.backup_dirs?.length > 0 ? (
                    <List
                      dataSource={config.backup_dirs}
                      renderItem={(dir: string) => (
                        <List.Item
                          actions={[
                            <Button
                              type="text"
                              danger
                              icon={<DeleteOutlined />}
                              onClick={() => handleRemoveDirectory(dir)}
                            />
                          ]}
                        >
                          <List.Item.Meta
                            avatar={<FolderOutlined style={{ fontSize: 24, color: '#faad14' }} />}
                            title={dir}
                          />
                        </List.Item>
                      )}
                    />
                  ) : (
                    <Empty
                      description="Henüz yedekleme dizini eklenmedi"
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                    />
                  )}
                </Card>
              </Col>
            </Row>
          )}

          {activeTab === 'restore' && (
            <Spin spinning={catalogLoading || catalogRecovering} tip={catalogRecovering ? "Katalog kurtarılıyor..." : "Katalog yükleniyor..."}>
              {/* Catalog Recovery Alert */}
              {!hasLocalCatalog && config?.device_id && (
                <Alert
                  message="Lokal Katalog Bulunamadı"
                  description="Bu cihazda yedekleme katalogu bulunamadı. Daha önce yedekleme yaptıysanız, sunucudan katalogu kurtarabilirsiniz."
                  type="warning"
                  showIcon
                  icon={<CloudDownloadOutlined />}
                  style={{ marginBottom: 16 }}
                  action={
                    <Button
                      type="primary"
                      onClick={handleRecoverCatalog}
                      loading={catalogRecovering}
                    >
                      Katalogu Kurtar
                    </Button>
                  }
                />
              )}

              {/* Time Machine Style Layout */}
              <div style={{ display: 'flex', height: 'calc(100vh - 180px)', gap: 16 }}>
                {/* Timeline - Left */}
                <div style={{
                  width: 180,
                  background: 'linear-gradient(180deg, #001529 0%, #002140 100%)',
                  borderRadius: 8,
                  padding: '12px 8px',
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center'
                }}>
                  {/* Header with zoom controls */}
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    width: '100%',
                    marginBottom: 8,
                    padding: '0 4px'
                  }}>
                    <Text style={{ color: '#fff', fontSize: 11 }}>
                      <CalendarOutlined /> Zaman
                    </Text>
                    <Space size={4}>
                      <Button
                        type="text"
                        size="small"
                        icon={<ZoomOutOutlined />}
                        onClick={() => setTimelineZoom(Math.max(5, timelineZoom - 5))}
                        style={{ color: '#fff', padding: 2 }}
                        disabled={timelineZoom <= 5}
                      />
                      <Text style={{ color: 'rgba(255,255,255,0.6)', fontSize: 10 }}>
                        {Math.min(timelineZoom, backupDates.length)}/{backupDates.length}
                      </Text>
                      <Button
                        type="text"
                        size="small"
                        icon={<ZoomInOutlined />}
                        onClick={() => setTimelineZoom(Math.min(backupDates.length, timelineZoom + 5))}
                        style={{ color: '#fff', padding: 2 }}
                        disabled={timelineZoom >= backupDates.length}
                      />
                    </Space>
                  </div>

                  <div style={{ flex: 1, overflow: 'auto', width: '100%' }}>
                    {backupDates.length > 0 ? (
                      backupDates.slice(0, timelineZoom).map((timestamp, index) => {
                        // Parse timestamp: "2024-01-15 14:30:05"
                        const parts = timestamp.split(' ');
                        const datePart = parts[0] || '';
                        const timePart = parts[1] || '';

                        return (
                          <div
                            key={timestamp}
                            onClick={() => {
                              console.log('[Timeline] Selecting date:', timestamp);
                              setSelectedDate(timestamp);
                            }}
                            style={{
                              cursor: 'pointer',
                              padding: '6px 8px',
                              marginBottom: 2,
                              borderRadius: 4,
                              background: timestamp === selectedDate ? '#1890ff' : 'transparent',
                              color: timestamp === selectedDate ? '#fff' : 'rgba(255,255,255,0.7)',
                              fontWeight: timestamp === selectedDate ? 600 : 400,
                              fontSize: 11,
                              transition: 'all 0.2s',
                              borderLeft: timestamp === selectedDate ? '3px solid #69c0ff' : '3px solid transparent'
                            }}
                          >
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                              <span>{datePart}</span>
                              <span style={{
                                fontSize: 10,
                                opacity: timestamp === selectedDate ? 1 : 0.7,
                                fontFamily: 'monospace'
                              }}>
                                {timePart}
                              </span>
                            </div>
                            {index === 0 && (
                              <div style={{ fontSize: 9, opacity: 0.6, textAlign: 'right' }}>Son yedek</div>
                            )}
                          </div>
                        );
                      })
                    ) : (
                      <Text style={{ color: 'rgba(255,255,255,0.5)', fontSize: 11, textAlign: 'center', display: 'block' }}>
                        Yedek yok
                      </Text>
                    )}
                    {backupDates.length > timelineZoom && (
                      <div style={{
                        textAlign: 'center',
                        padding: '8px',
                        color: 'rgba(255,255,255,0.5)',
                        fontSize: 10
                      }}>
                        +{backupDates.length - timelineZoom} daha...
                        <br />
                        <Button
                          type="link"
                          size="small"
                          onClick={() => setTimelineZoom(backupDates.length)}
                          style={{ color: '#69c0ff', fontSize: 10, padding: 0 }}
                        >
                          Tümünü göster
                        </Button>
                      </div>
                    )}
                  </div>
                  <Button
                    size="small"
                    icon={<ReloadOutlined />}
                    onClick={loadTimeMachineData}
                    loading={catalogLoading}
                    style={{ marginTop: 8 }}
                  >
                    Yenile
                  </Button>
                </div>

                {/* File Browser - Right */}
                <Card
                  title={
                    <Space>
                      <FolderOutlined />
                      {/* Breadcrumb navigation */}
                      {breadcrumbParts.map((part, idx) => (
                        <span key={part.path}>
                          {idx > 0 && <span style={{ margin: '0 4px', color: '#999' }}>/</span>}
                          <span
                            onClick={() => navigateToDir(part.path)}
                            style={{
                              cursor: 'pointer',
                              color: idx === breadcrumbParts.length - 1 ? '#1890ff' : '#666',
                              fontWeight: idx === breadcrumbParts.length - 1 ? 600 : 400
                            }}
                          >
                            {idx === 0 ? <HomeOutlined /> : part.name}
                          </span>
                        </span>
                      ))}
                      <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                        ({browserItems.length} öğe, toplam {catalogFiles.length} dosya)
                      </Text>
                    </Space>
                  }
                  extra={
                    <Space>
                      {currentPath !== '/' && (
                        <Button
                          icon={<ArrowLeftOutlined />}
                          onClick={navigateUp}
                        >
                          Geri
                        </Button>
                      )}
                      <Button
                        type="primary"
                        icon={<DownloadOutlined />}
                        onClick={() => handleRestoreDirectory(currentPath)}
                        loading={restoreLoading}
                        disabled={browserItems.length === 0 || !selectedDate}
                        title={selectedDate ? `Seçili tarih: ${selectedDate}` : 'Tarih seçin'}
                      >
                        Bu Dizini Geri Yükle ({selectedDate ? selectedDate.split(' ')[1] : '?'})
                      </Button>
                    </Space>
                  }
                  style={{ flex: 1, overflow: 'hidden' }}
                  styles={{ body: { height: 'calc(100% - 57px)', overflow: 'auto', padding: 0 } }}
                >
                  {browserItems.length > 0 ? (
                    <List
                      size="small"
                      dataSource={browserItems}
                      renderItem={(item) => (
                        <List.Item
                          style={{
                            padding: '8px 16px',
                            cursor: 'pointer',
                            background: selectedFile?.orig_path === item.path ? '#e6f7ff' : 'transparent'
                          }}
                          onClick={() => {
                            if (item.isDir) {
                              navigateToDir(item.path);
                            } else if (item.file) {
                              handleSelectFile(item.file);
                            }
                          }}
                          actions={[
                            <Button
                              type="primary"
                              size="small"
                              icon={<DownloadOutlined />}
                              onClick={(e) => {
                                e.stopPropagation();
                                if (item.isDir) {
                                  handleRestoreDirectory(item.path);
                                } else if (item.file) {
                                  handleRestoreFileVersion(item.file, { timestamp: item.file.latest_version, size: item.file.size, hash: '' });
                                }
                              }}
                              loading={restoreLoading}
                            >
                              Geri Yükle
                            </Button>
                          ]}
                        >
                          <List.Item.Meta
                            avatar={
                              item.isDir
                                ? <FolderOutlined style={{ fontSize: 24, color: '#faad14' }} />
                                : <FileOutlined style={{ fontSize: 24, color: '#1890ff' }} />
                            }
                            title={
                              <span style={{ fontWeight: item.isDir ? 600 : 400 }}>
                                {item.name}
                                {item.isDir && <span style={{ color: '#999', marginLeft: 4 }}>/</span>}
                              </span>
                            }
                            description={
                              <Space size="small">
                                <Tag style={{ fontSize: 10 }}>{formatBytes(item.size || 0)}</Tag>
                                {item.isDir && item.fileCount && (
                                  <Tag color="blue" style={{ fontSize: 10 }}>{item.fileCount} dosya</Tag>
                                )}
                                {!item.isDir && item.file && (
                                  <Tag color="green" style={{ fontSize: 10 }}>{item.file.version_count} versiyon</Tag>
                                )}
                              </Space>
                            }
                          />
                        </List.Item>
                      )}
                    />
                  ) : (
                    <Empty
                      description={catalogFiles.length === 0 ? "Katalogda dosya bulunamadı" : "Bu dizinde dosya yok"}
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      style={{ marginTop: 60 }}
                    />
                  )}
                </Card>
              </div>
            </Spin>
          )}

          {activeTab === 'settings' && (
            <Row gutter={[24, 24]}>
              <Col span={12}>
                <Card title="Sunucu Ayarları">
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <div>
                      <Text type="secondary">Sunucu URL</Text>
                      <Input value={config?.server_url} disabled />
                    </div>
                    <div>
                      <Text type="secondary">Cihaz Adı</Text>
                      <Input value={config?.device_name} disabled />
                    </div>
                  </Space>
                </Card>
              </Col>
              <Col span={12}>
                <Card title="Şifreleme">
                  <Space direction="vertical" style={{ width: '100%' }}>
                    {config?.encryption_key ? (
                      <Alert
                        message="Şifreleme Aktif"
                        description="Dosyalarınız AES-256 ile şifrelenerek yedekleniyor. Şifreleme anahtarınızı güvenli bir yerde saklayın."
                        type="success"
                        showIcon
                        icon={<LockOutlined />}
                      />
                    ) : (
                      <Alert
                        message="Şifreleme Ayarlanmadı"
                        description="Yedekleme yapmadan önce şifreleme anahtarı belirlemeniz gerekiyor."
                        type="warning"
                        showIcon
                        action={
                          <Button
                            size="small"
                            type="primary"
                            onClick={() => setEncryptionKeyModal(true)}
                          >
                            Ayarla
                          </Button>
                        }
                      />
                    )}
                  </Space>
                </Card>
              </Col>
              <Col span={12}>
                <Card title="Kara Liste">
                  <Text type="secondary">Yedeklenmeyen dosya uzantıları:</Text>
                  <div style={{ marginTop: 8 }}>
                    {config?.blacklist?.map((ext: string) => (
                      <Tag key={ext}>{ext}</Tag>
                    ))}
                  </div>
                </Card>
              </Col>
            </Row>
          )}
        </Content>
      </Layout>

      {/* Encryption Key Setup Modal */}
      <Modal
        title={
          <Space>
            <KeyOutlined />
            <span>Şifreleme Anahtarı Ayarla</span>
          </Space>
        }
        open={encryptionKeyModal}
        onOk={handleSaveEncryptionKey}
        onCancel={() => {
          setEncryptionKeyModal(false);
          setEncryptionKey('');
          setEncryptionKeyConfirm('');
        }}
        okText="Kaydet"
        cancelText="İptal"
      >
        <Alert
          message="Önemli"
          description="Bu anahtar dosyalarınızı şifrelemek için kullanılacak. Anahtarınızı kaybederseniz yedeklerinize erişemezsiniz!"
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Form layout="vertical">
          <Form.Item label="Şifreleme Anahtarı" required>
            <Input.Password
              placeholder="En az 8 karakter"
              value={encryptionKey}
              onChange={(e) => setEncryptionKey(e.target.value)}
              prefix={<LockOutlined />}
            />
          </Form.Item>
          <Form.Item label="Anahtar Tekrar" required>
            <Input.Password
              placeholder="Anahtarı tekrar girin"
              value={encryptionKeyConfirm}
              onChange={(e) => setEncryptionKeyConfirm(e.target.value)}
              prefix={<LockOutlined />}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Device Registration Modal */}
      <Modal
        title={
          <Space>
            <HddOutlined />
            <span>Cihaz Kaydı</span>
          </Space>
        }
        open={deviceModal}
        onOk={handleRegisterDevice}
        onCancel={() => {
          setDeviceModal(false);
          setDeviceName('');
        }}
        okText="Kaydet"
        cancelText="İptal"
      >
        <Alert
          message="Cihaz Kaydı Gerekli"
          description="Yedekleme yapmak için bu cihazı sunucuya kaydetmeniz gerekiyor. Cihazınız için tanımlayıcı bir isim girin."
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Form layout="vertical">
          <Form.Item label="Cihaz Adı" required>
            <Input
              placeholder="Örn: Ev Bilgisayarım, İş Laptopu"
              value={deviceName}
              onChange={(e) => setDeviceName(e.target.value)}
              prefix={<HddOutlined />}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  );
}
