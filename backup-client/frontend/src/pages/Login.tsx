import { useState, useEffect } from 'react';
import { Form, Input, Button, Card, message, Typography, InputNumber, Modal } from 'antd';
import { UserOutlined, LockOutlined, CloudServerOutlined, MailOutlined, SettingOutlined } from '@ant-design/icons';

const { Title, Text, Link } = Typography;

interface LoginProps {
  onLogin: () => void;
}

// Mock functions for development
const mockLogin = async (email: string, password: string) => {
  await new Promise(resolve => setTimeout(resolve, 1000));
  if (email && password) {
    localStorage.setItem('token', 'mock-token');
    return { success: true, user: { name: email }, token: 'mock-token' };
  }
  throw new Error('Invalid credentials');
};

const mockRegister = async (name: string, email: string, password: string, plan: number) => {
  await new Promise(resolve => setTimeout(resolve, 1000));
  if (name && email && password && plan) {
    localStorage.setItem('token', 'mock-token');
    return { success: true, user: { name }, token: 'mock-token' };
  }
  throw new Error('Registration failed');
};

const mockGetServerURL = async () => {
  return localStorage.getItem('serverURL') || 'https://ozet.mynodes.xyz';
};

const mockSetServerURL = async (url: string) => {
  localStorage.setItem('serverURL', url);
  return true;
};

const Login = (window as any).go?.main?.App?.Login || mockLogin;
const Register = (window as any).go?.main?.App?.Register || mockRegister;
const GetServerURL = (window as any).go?.main?.App?.GetServerURL || mockGetServerURL;
const SetServerURL = (window as any).go?.main?.App?.SetServerURL || mockSetServerURL;

export default function LoginPage({ onLogin }: LoginProps) {
  const [loading, setLoading] = useState(false);
  const [isRegister, setIsRegister] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [serverURL, setServerURL] = useState('');
  const [savingSettings, setSavingSettings] = useState(false);

  useEffect(() => {
    loadServerURL();
  }, []);

  const loadServerURL = async () => {
    try {
      const url = await GetServerURL();
      setServerURL(url || '');
      // Auto-open settings if server URL is not configured
      if (!url || url.trim() === '') {
        setSettingsOpen(true);
      }
    } catch (err) {
      console.error('Failed to load server URL', err);
      // Open settings on error too
      setSettingsOpen(true);
    }
  };

  const handleSaveSettings = async () => {
    if (!serverURL.trim()) {
      message.error('Sunucu adresi gerekli');
      return;
    }
    setSavingSettings(true);
    try {
      await SetServerURL(serverURL.trim());
      message.success('Ayarlar kaydedildi');
      setSettingsOpen(false);
    } catch (err: any) {
      message.error(err.message || 'Ayarlar kaydedilemedi');
    } finally {
      setSavingSettings(false);
    }
  };

  const handleLogin = async (values: { email: string; password: string }) => {
    if (!serverURL || serverURL.trim() === '') {
      message.warning('Lutfen once sunucu adresini ayarlayin');
      setSettingsOpen(true);
      return;
    }
    setLoading(true);
    try {
      await Login(values.email, values.password);
      message.success('Giris basarili!');
      onLogin();
    } catch (err: any) {
      message.error(err.message || 'Giris basarisiz');
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (values: { name: string; email: string; password: string; plan: number }) => {
    if (!serverURL || serverURL.trim() === '') {
      message.warning('Lutfen once sunucu adresini ayarlayin');
      setSettingsOpen(true);
      return;
    }
    setLoading(true);
    try {
      await Register(values.name, values.email, values.password, values.plan);
      message.success('Kayit basarili! Yonetici onayi bekleniyor.');
      onLogin();
    } catch (err: any) {
      message.error(err.message || 'Kayit basarisiz');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    }}>
      <Card style={{ width: 400, boxShadow: '0 4px 12px rgba(0,0,0,0.15)' }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <CloudServerOutlined style={{ fontSize: 48, color: '#1890ff' }} />
          <Title level={3} style={{ marginTop: 16, marginBottom: 4 }}>
            Backup Client
          </Title>
          <Text type="secondary">
            {isRegister ? 'Yeni hesap olustur' : 'Guvenli yedekleme sistemi'}
          </Text>
        </div>

        {!isRegister ? (
          <Form
            name="login"
            onFinish={handleLogin}
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              name="email"
              rules={[
                { required: true, message: 'E-posta gerekli' },
                { type: 'email', message: 'Gecerli e-posta girin' }
              ]}
            >
              <Input
                prefix={<MailOutlined />}
                placeholder="E-posta"
                size="large"
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: 'Sifre gerekli' }]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="Sifre"
                size="large"
              />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                block
                size="large"
              >
                Giris Yap
              </Button>
            </Form.Item>

            <div style={{ textAlign: 'center', marginBottom: 12 }}>
              <Text>Hesabiniz yok mu? </Text>
              <Link onClick={() => setIsRegister(true)}>Kayit Ol</Link>
            </div>
          </Form>
        ) : (
          <Form
            name="register"
            onFinish={handleRegister}
            layout="vertical"
            requiredMark={false}
            initialValues={{ plan: 10 }}
          >
            <Form.Item
              name="name"
              rules={[{ required: true, message: 'Ad Soyad gerekli' }]}
            >
              <Input
                prefix={<UserOutlined />}
                placeholder="Ad Soyad"
                size="large"
              />
            </Form.Item>

            <Form.Item
              name="email"
              rules={[
                { required: true, message: 'E-posta gerekli' },
                { type: 'email', message: 'Gecerli e-posta girin' }
              ]}
            >
              <Input
                prefix={<MailOutlined />}
                placeholder="E-posta"
                size="large"
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[
                { required: true, message: 'Sifre gerekli' },
                { min: 6, message: 'En az 6 karakter' }
              ]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="Sifre"
                size="large"
              />
            </Form.Item>

            <Form.Item
              name="plan"
              label="Plan (GB)"
              rules={[{ required: true, message: 'Plan secin' }]}
            >
              <InputNumber
                min={1}
                max={100}
                size="large"
                style={{ width: '100%' }}
              />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                block
                size="large"
              >
                Kayit Ol
              </Button>
            </Form.Item>

            <div style={{ textAlign: 'center', marginBottom: 12 }}>
              <Text>Zaten hesabiniz var mi? </Text>
              <Link onClick={() => setIsRegister(false)}>Giris Yap</Link>
            </div>
          </Form>
        )}

        <div style={{ textAlign: 'center', borderTop: '1px solid #f0f0f0', paddingTop: 12 }}>
          <Button
            type="link"
            icon={<SettingOutlined />}
            onClick={() => setSettingsOpen(true)}
          >
            Sunucu Ayarlari
          </Button>
          {serverURL && (
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                <CloudServerOutlined /> {serverURL}
              </Text>
            </div>
          )}
        </div>
      </Card>

      <Modal
        title="Sunucu Ayarlari"
        open={settingsOpen}
        onCancel={() => {
          // Only allow closing if server URL is set
          if (serverURL && serverURL.trim() !== '') {
            setSettingsOpen(false);
          } else {
            message.warning('Devam etmek icin sunucu adresi gerekli');
          }
        }}
        closable={!!(serverURL && serverURL.trim() !== '')}
        maskClosable={!!(serverURL && serverURL.trim() !== '')}
        footer={[
          <Button
            key="cancel"
            onClick={() => setSettingsOpen(false)}
            disabled={!serverURL || serverURL.trim() === ''}
          >
            Iptal
          </Button>,
          <Button key="save" type="primary" loading={savingSettings} onClick={handleSaveSettings}>
            Kaydet
          </Button>
        ]}
      >
        <Form layout="vertical">
          <Form.Item label="Sunucu Adresi" required>
            <Input
              placeholder="https://ozet.mynodes.xyz"
              value={serverURL}
              onChange={(e) => setServerURL(e.target.value)}
              prefix={<CloudServerOutlined />}
            />
          </Form.Item>
          <Text type="secondary">
            Yedekleme sunucusunun tam adresini girin. Ornegin: https://ozet.mynodes.xyz
          </Text>
        </Form>
      </Modal>
    </div>
  );
}
