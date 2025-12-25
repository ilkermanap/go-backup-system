import { useState } from 'react';
import { Form, Input, Button, Card, message, Typography } from 'antd';
import { UserOutlined, LockOutlined, CloudServerOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

interface LoginProps {
  onLogin: () => void;
}

// Mock login for development
const mockLogin = async (email: string, password: string) => {
  // Simulate API call
  await new Promise(resolve => setTimeout(resolve, 1000));
  if (email && password) {
    localStorage.setItem('token', 'mock-token');
    return { success: true, user: { name: email }, token: 'mock-token' };
  }
  throw new Error('Invalid credentials');
};

const Login = (window as any).go?.main?.App?.Login || mockLogin;

export default function LoginPage({ onLogin }: LoginProps) {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (values: { email: string; password: string }) => {
    setLoading(true);
    try {
      await Login(values.email, values.password);
      message.success('Giriş başarılı!');
      onLogin();
    } catch (err: any) {
      message.error(err.message || 'Giriş başarısız');
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
          <Text type="secondary">Güvenli yedekleme sistemi</Text>
        </div>

        <Form
          name="login"
          onFinish={handleSubmit}
          layout="vertical"
          requiredMark={false}
        >
          <Form.Item
            name="email"
            rules={[
              { required: true, message: 'E-posta gerekli' },
              { type: 'email', message: 'Geçerli e-posta girin' }
            ]}
          >
            <Input
              prefix={<UserOutlined />}
              placeholder="E-posta"
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: 'Şifre gerekli' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="Şifre"
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
              Giriş Yap
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
