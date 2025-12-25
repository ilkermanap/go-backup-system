import { useEffect, useState } from 'react';
import {
  Table, Button, Space, Tag, message, Card, Typography, Popconfirm, Modal, Form, Input, Select, InputNumber, Progress, Row, Col
} from 'antd';
import {
  CheckOutlined, DeleteOutlined, ExclamationCircleOutlined, CrownOutlined, UserOutlined, PlusOutlined,
  EditOutlined, KeyOutlined, StopOutlined, PlayCircleOutlined, SearchOutlined, ReloadOutlined
} from '@ant-design/icons';
import { getUsers, approveUser, deleteUser, createUser, updateUser, resetPassword, toggleUserStatus, bulkDeleteUsers } from '../api';
import type { UserFilters } from '../api';

const { Title } = Typography;
const { Search } = Input;

interface User {
  id: number;
  name: string;
  email: string;
  role: 'admin' | 'user';
  plan: number;
  used_space: number;
  is_approved: boolean;
  is_active: boolean;
  created_at: string;
}

export default function Users() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });
  const [filters, setFilters] = useState<UserFilters>({ page: 1, perPage: 20 });
  const [statusFilter, setStatusFilter] = useState<string | undefined>(undefined);

  // Modals
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  // Forms
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();
  const [passwordForm] = Form.useForm();
  const [creating, setCreating] = useState(false);
  const [updating, setUpdating] = useState(false);

  const fetchUsers = async (newFilters?: UserFilters) => {
    setLoading(true);
    try {
      const currentFilters = newFilters || filters;
      const response = await getUsers(currentFilters);
      setUsers(response.data.data);
      setPagination({
        current: response.data.meta.page,
        pageSize: response.data.meta.per_page,
        total: response.data.meta.total
      });
    } catch (error) {
      message.error('Kullanicilar yuklenemedi');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleSearch = (value: string) => {
    const newFilters = { ...filters, search: value || undefined, page: 1 };
    setFilters(newFilters);
    fetchUsers(newFilters);
  };

  const handleStatusFilter = (value: string | undefined) => {
    setStatusFilter(value);
    const newFilters = { ...filters, status: value as UserFilters['status'], page: 1 };
    setFilters(newFilters);
    fetchUsers(newFilters);
  };

  const handleTableChange = (pag: any, _: any, sorter: any) => {
    const newFilters: UserFilters = {
      ...filters,
      page: pag.current,
      perPage: pag.pageSize,
      sort: sorter.field,
      order: sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : undefined
    };
    setFilters(newFilters);
    fetchUsers(newFilters);
  };

  const handleApprove = async (id: number) => {
    try {
      await approveUser(id);
      message.success('Kullanici onaylandi');
      fetchUsers();
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Onay basarisiz');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteUser(id);
      message.success('Kullanici silindi');
      fetchUsers();
    } catch (error) {
      message.error('Silme basarisiz');
    }
  };

  const handleBulkDelete = async () => {
    try {
      await bulkDeleteUsers(selectedRowKeys as number[]);
      message.success(`${selectedRowKeys.length} kullanici silindi`);
      setSelectedRowKeys([]);
      fetchUsers();
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Toplu silme basarisiz');
    }
  };

  const handleCreate = async (values: { name: string; email: string; password: string; plan: number; role: string }) => {
    setCreating(true);
    try {
      await createUser(values);
      message.success('Kullanici olusturuldu');
      setIsCreateModalOpen(false);
      createForm.resetFields();
      fetchUsers();
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Olusturma basarisiz');
    } finally {
      setCreating(false);
    }
  };

  const handleEdit = async (values: { name: string; plan: number }) => {
    if (!selectedUser) return;
    setUpdating(true);
    try {
      await updateUser(selectedUser.id, values);
      message.success('Kullanici guncellendi');
      setIsEditModalOpen(false);
      editForm.resetFields();
      setSelectedUser(null);
      fetchUsers();
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Guncelleme basarisiz');
    } finally {
      setUpdating(false);
    }
  };

  const handleResetPassword = async (values: { password: string }) => {
    if (!selectedUser) return;
    setUpdating(true);
    try {
      await resetPassword(selectedUser.id, values.password);
      message.success('Parola sifirlandi');
      setIsPasswordModalOpen(false);
      passwordForm.resetFields();
      setSelectedUser(null);
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Parola sifirlama basarisiz');
    } finally {
      setUpdating(false);
    }
  };

  const handleToggleStatus = async (id: number) => {
    try {
      await toggleUserStatus(id);
      message.success('Kullanici durumu degistirildi');
      fetchUsers();
    } catch (error: any) {
      message.error(error.response?.data?.error?.message || 'Durum degistirme basarisiz');
    }
  };

  const openEditModal = (user: User) => {
    setSelectedUser(user);
    editForm.setFieldsValue({ name: user.name, plan: user.plan });
    setIsEditModalOpen(true);
  };

  const openPasswordModal = (user: User) => {
    setSelectedUser(user);
    setIsPasswordModalOpen(true);
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60, sorter: true },
    { title: 'Ad', dataIndex: 'name', key: 'name', sorter: true },
    { title: 'E-posta', dataIndex: 'email', key: 'email', sorter: true },
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
      title: 'Kota Kullanimi',
      key: 'quota',
      width: 200,
      render: (_: any, record: User) => {
        const usedGB = record.used_space / (1024 * 1024 * 1024);
        const percent = Math.min((usedGB / record.plan) * 100, 100);
        return (
          <div>
            <Progress
              percent={percent}
              size="small"
              status={percent >= 90 ? 'exception' : 'normal'}
              format={() => `${formatBytes(record.used_space)} / ${record.plan} GB`}
            />
          </div>
        );
      }
    },
    {
      title: 'Durum',
      key: 'status',
      render: (_: any, record: User) => (
        <Space>
          <Tag color={record.is_approved ? 'green' : 'orange'}>
            {record.is_approved ? 'Onayli' : 'Bekliyor'}
          </Tag>
          <Tag color={record.is_active ? 'blue' : 'red'}>
            {record.is_active ? 'Aktif' : 'Pasif'}
          </Tag>
        </Space>
      )
    },
    { title: 'Kayit Tarihi', dataIndex: 'created_at', key: 'created_at', sorter: true },
    {
      title: 'Islemler',
      key: 'actions',
      width: 280,
      render: (_: any, record: User) => (
        <Space wrap>
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
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => openEditModal(record)}
          >
            Duzenle
          </Button>
          <Button
            size="small"
            icon={<KeyOutlined />}
            onClick={() => openPasswordModal(record)}
          >
            Parola
          </Button>
          {record.role !== 'admin' && (
            <>
              <Button
                size="small"
                icon={record.is_active ? <StopOutlined /> : <PlayCircleOutlined />}
                onClick={() => handleToggleStatus(record.id)}
                danger={record.is_active}
              >
                {record.is_active ? 'Devre Disi' : 'Aktif Et'}
              </Button>
              <Popconfirm
                title="Kullaniciyi silmek istediginize emin misiniz?"
                onConfirm={() => handleDelete(record.id)}
                okText="Evet"
                cancelText="Hayir"
                icon={<ExclamationCircleOutlined style={{ color: 'red' }} />}
              >
                <Button danger size="small" icon={<DeleteOutlined />}>
                  Sil
                </Button>
              </Popconfirm>
            </>
          )}
        </Space>
      )
    }
  ];

  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => setSelectedRowKeys(keys),
    getCheckboxProps: (record: User) => ({
      disabled: record.role === 'admin'
    })
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>Kullanicilar</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsCreateModalOpen(true)}>
          Kullanici Ekle
        </Button>
      </div>

      <Card style={{ marginBottom: 16 }}>
        <Row gutter={16} align="middle">
          <Col flex="auto">
            <Search
              placeholder="Ad veya e-posta ile ara..."
              allowClear
              enterButton={<SearchOutlined />}
              onSearch={handleSearch}
              style={{ maxWidth: 400 }}
            />
          </Col>
          <Col>
            <Select
              placeholder="Durum filtresi"
              allowClear
              style={{ width: 150 }}
              value={statusFilter}
              onChange={handleStatusFilter}
            >
              <Select.Option value="approved">Onayli</Select.Option>
              <Select.Option value="pending">Bekleyen</Select.Option>
              <Select.Option value="active">Aktif</Select.Option>
              <Select.Option value="inactive">Pasif</Select.Option>
            </Select>
          </Col>
          <Col>
            <Button icon={<ReloadOutlined />} onClick={() => fetchUsers()}>
              Yenile
            </Button>
          </Col>
          {selectedRowKeys.length > 0 && (
            <Col>
              <Popconfirm
                title={`${selectedRowKeys.length} kullaniciyi silmek istediginize emin misiniz?`}
                onConfirm={handleBulkDelete}
                okText="Evet"
                cancelText="Hayir"
                icon={<ExclamationCircleOutlined style={{ color: 'red' }} />}
              >
                <Button danger icon={<DeleteOutlined />}>
                  Secilenleri Sil ({selectedRowKeys.length})
                </Button>
              </Popconfirm>
            </Col>
          )}
        </Row>
      </Card>

      <Card>
        <Table
          dataSource={users}
          columns={columns}
          rowKey="id"
          loading={loading}
          rowSelection={rowSelection}
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showTotal: (total) => `Toplam ${total} kullanici`
          }}
          onChange={handleTableChange}
          scroll={{ x: 1200 }}
        />
      </Card>

      {/* Create User Modal */}
      <Modal
        title="Yeni Kullanici"
        open={isCreateModalOpen}
        onCancel={() => { setIsCreateModalOpen(false); createForm.resetFields(); }}
        footer={null}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="Ad Soyad" rules={[{ required: true, message: 'Ad gerekli' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="email" label="E-posta" rules={[{ required: true, type: 'email', message: 'Gecerli e-posta gerekli' }]}>
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
              <Select.Option value="user">Kullanici</Select.Option>
              <Select.Option value="admin">Admin</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={creating}>
                Olustur
              </Button>
              <Button onClick={() => { setIsCreateModalOpen(false); createForm.resetFields(); }}>
                Iptal
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit User Modal */}
      <Modal
        title="Kullaniciyi Duzenle"
        open={isEditModalOpen}
        onCancel={() => { setIsEditModalOpen(false); editForm.resetFields(); setSelectedUser(null); }}
        footer={null}
      >
        <Form form={editForm} layout="vertical" onFinish={handleEdit}>
          <Form.Item name="name" label="Ad Soyad" rules={[{ required: true, message: 'Ad gerekli' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="plan" label="Plan (GB)" rules={[{ required: true, message: 'Plan gerekli' }]}>
            <InputNumber min={1} max={200} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={updating}>
                Kaydet
              </Button>
              <Button onClick={() => { setIsEditModalOpen(false); editForm.resetFields(); setSelectedUser(null); }}>
                Iptal
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* Reset Password Modal */}
      <Modal
        title="Parola Sifirla"
        open={isPasswordModalOpen}
        onCancel={() => { setIsPasswordModalOpen(false); passwordForm.resetFields(); setSelectedUser(null); }}
        footer={null}
      >
        <p>Kullanici: <strong>{selectedUser?.name}</strong> ({selectedUser?.email})</p>
        <Form form={passwordForm} layout="vertical" onFinish={handleResetPassword}>
          <Form.Item name="password" label="Yeni Parola" rules={[{ required: true, min: 6, message: 'En az 6 karakter' }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="confirmPassword" label="Parola Tekrar"
            rules={[
              { required: true, message: 'Parola tekrari gerekli' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('Parolalar eslesmiyor'));
                },
              }),
            ]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={updating}>
                Sifirla
              </Button>
              <Button onClick={() => { setIsPasswordModalOpen(false); passwordForm.resetFields(); setSelectedUser(null); }}>
                Iptal
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
