import { useEffect, useState } from 'react';
import {
  Table, Button, Space, Tag, message, Card, Typography, Popconfirm
} from 'antd';
import {
  CheckOutlined, DeleteOutlined, ExclamationCircleOutlined, CrownOutlined, UserOutlined
} from '@ant-design/icons';
import { getUsers, approveUser, deleteUser } from '../api';

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
      <Title level={2}>Kullanıcılar</Title>
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
    </div>
  );
}
