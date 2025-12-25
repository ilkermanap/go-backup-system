import { useEffect, useState } from 'react';
import {
  Table, Button, Space, Tag, message, Card, Typography, Popconfirm, Modal, Form, Input, Select, InputNumber
} from 'antd';
import {
  CheckOutlined, DeleteOutlined, ExclamationCircleOutlined, CrownOutlined, UserOutlined, PlusOutlined
} from '@ant-design/icons';
import { getUsers, approveUser, deleteUser, createUser } from '../api';

const { Title } = Typography;

interface User {
  id: number;
  name: string;
  email: string;
  role: 'admin' | 'user';
  plan: number;
  is_approved: boolean;
  created_at: string;
}

export default function Users() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const fetchUsers = async (page = 1, pageSize = 20) => {
    setLoading(true);
    try {
      const response = await getUsers(page, pageSize);
      setUsers(response.data.data);
      setPagination({
        current: response.data.meta.page,
        pageSize: response.data.meta.per_page,
        total: response.data.meta.total
      });
    } catch (error) {
      message.error('Kullanıcılar yüklenemedi');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleApprove = async (id: number) => {
    try {
      await approveUser(id);
      message.success('Kullanıcı onaylandı');
      fetchUsers(pagination.current, pagination.pageSize);
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Onay başarısız');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteUser(id);
      message.success('Kullanıcı silindi');
      fetchUsers(pagination.current, pagination.pageSize);
    } catch (error) {
      message.error('Silme başarısız');
    }
  };

  const handleCreate = async (values: { name: string; email: string; password: string; plan: number; role: string }) => {
    setCreating(true);
    try {
      await createUser(values);
      message.success('Kullanıcı oluşturuldu');
      setIsModalOpen(false);
      form.resetFields();
      fetchUsers(pagination.current, pagination.pageSize);
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Oluşturma başarısız');
    } finally {
      setCreating(false);
    }
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: 'Ad', dataIndex: 'name', key: 'name' },
    { title: 'E-posta', dataIndex: 'email', key: 'email' },
    {
      title: 'Rol',
      dataIndex: 'role',
      key: 'role',
      render: (role: string) => (
        <Tag
          icon={role === 'admin' ? <CrownOutlined /> : <UserOutlined />}
          color={role === 'admin' ? 'gold' : 'blue'}
        >
          {role === 'admin' ? 'Admin' : 'User'}
        </Tag>
      )
    },
    {
      title: 'Plan',
      dataIndex: 'plan',
      key: 'plan',
      render: (plan: number) => `${plan} GB`
    },
    {
      title: 'Durum',
      dataIndex: 'is_approved',
      key: 'is_approved',
      render: (approved: boolean) => (
        <Tag color={approved ? 'green' : 'orange'}>
          {approved ? 'Onaylı' : 'Bekliyor'}
        </Tag>
      )
    },
    { title: 'Kayıt Tarihi', dataIndex: 'created_at', key: 'created_at' },
    {
      title: 'İşlemler',
      key: 'actions',
      render: (_: any, record: User) => (
        <Space>
          {!record.is_approved && (
            <Button
              type="primary"
              size="small"
              icon={<CheckOutlined />}
              onClick={() => handleApprove(record.id)}
            >
              Onayla
            </Button>
          )}
          {record.role !== 'admin' && (
            <Popconfirm
              title="Kullanıcıyı silmek istediğinize emin misiniz?"
              onConfirm={() => handleDelete(record.id)}
              okText="Evet"
              cancelText="Hayır"
              icon={<ExclamationCircleOutlined style={{ color: 'red' }} />}
            >
              <Button danger size="small" icon={<DeleteOutlined />}>
                Sil
              </Button>
            </Popconfirm>
          )}
        </Space>
      )
    }
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>Kullanıcılar</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalOpen(true)}>
          Kullanıcı Ekle
        </Button>
      </div>
      <Card>
        <Table
          dataSource={users}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showTotal: (total) => `Toplam ${total} kullanıcı`
          }}
          onChange={(pag) => fetchUsers(pag.current, pag.pageSize)}
        />
      </Card>

      <Modal
        title="Yeni Kullanıcı"
        open={isModalOpen}
        onCancel={() => { setIsModalOpen(false); form.resetFields(); }}
        footer={null}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="Ad Soyad" rules={[{ required: true, message: 'Ad gerekli' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="email" label="E-posta" rules={[{ required: true, type: 'email', message: 'Geçerli e-posta gerekli' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="password" label="Parola" rules={[{ required: true, min: 6, message: 'En az 6 karakter' }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="plan" label="Plan (GB)" rules={[{ required: true, message: 'Plan gerekli' }]} initialValue={10}>
            <InputNumber min={1} max={200} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="role" label="Rol" initialValue="user">
            <Select>
              <Select.Option value="user">Kullanıcı</Select.Option>
              <Select.Option value="admin">Admin</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={creating}>
                Oluştur
              </Button>
              <Button onClick={() => { setIsModalOpen(false); form.resetFields(); }}>
                İptal
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
