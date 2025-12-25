import { useEffect, useState } from 'react';
import {
  Table, Button, Space, Card, Typography, Modal, Form, Input, message, Popconfirm, Tag
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, FolderOpenOutlined, DownloadOutlined
} from '@ant-design/icons';
import { getDevices, createDevice, deleteDevice, getBackups, downloadBackup } from '../api';

const { Title } = Typography;

interface Device {
  id: number;
  name: string;
  created_at: string;
}

interface Backup {
  id: number;
  file_name: string;
  file_size: number;
  size_mb: number;
  checksum: string;
  created_at: string;
}

export default function Devices() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalVisible, setModalVisible] = useState(false);
  const [backupModalVisible, setBackupModalVisible] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [backupsLoading, setBackupsLoading] = useState(false);
  const [form] = Form.useForm();

  const fetchDevices = async () => {
    setLoading(true);
    try {
      const response = await getDevices();
      setDevices(response.data.data);
    } catch (error) {
      message.error('Cihazlar yüklenemedi');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDevices();
  }, []);

  const handleCreate = async (values: { name: string }) => {
    try {
      await createDevice(values.name);
      message.success('Cihaz eklendi');
      setModalVisible(false);
      form.resetFields();
      fetchDevices();
    } catch (error) {
      message.error('Cihaz eklenemedi');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteDevice(id);
      message.success('Cihaz silindi');
      fetchDevices();
    } catch (error) {
      message.error('Silme başarısız');
    }
  };

  const showBackups = async (device: Device) => {
    setSelectedDevice(device);
    setBackupModalVisible(true);
    setBackupsLoading(true);
    try {
      const response = await getBackups(device.id);
      setBackups(response.data.data);
    } catch (error) {
      message.error('Yedekler yüklenemedi');
    } finally {
      setBackupsLoading(false);
    }
  };

  const handleDownload = (deviceId: number, backupId: number) => {
    const token = localStorage.getItem('token');
    const url = downloadBackup(deviceId, backupId);

    // Create a temporary link with auth header
    fetch(url, {
      headers: { Authorization: `Bearer ${token}` }
    })
      .then(res => res.blob())
      .then(blob => {
        const backup = backups.find(b => b.id === backupId);
        const link = document.createElement('a');
        link.href = URL.createObjectURL(blob);
        link.download = backup?.file_name || 'backup';
        link.click();
      });
  };

  const deviceColumns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: 'Ad', dataIndex: 'name', key: 'name' },
    { title: 'Oluşturulma', dataIndex: 'created_at', key: 'created_at' },
    {
      title: 'İşlemler',
      key: 'actions',
      render: (_: any, record: Device) => (
        <Space>
          <Button
            size="small"
            icon={<FolderOpenOutlined />}
            onClick={() => showBackups(record)}
          >
            Yedekler
          </Button>
          <Popconfirm
            title="Cihazı ve tüm yedeklerini silmek istediğinize emin misiniz?"
            onConfirm={() => handleDelete(record.id)}
            okText="Evet"
            cancelText="Hayır"
          >
            <Button danger size="small" icon={<DeleteOutlined />}>
              Sil
            </Button>
          </Popconfirm>
        </Space>
      )
    }
  ];

  const backupColumns = [
    { title: 'Dosya Adı', dataIndex: 'file_name', key: 'file_name' },
    {
      title: 'Boyut',
      dataIndex: 'size_mb',
      key: 'size_mb',
      render: (val: number) => `${val.toFixed(4)} MB`
    },
    {
      title: 'Checksum',
      dataIndex: 'checksum',
      key: 'checksum',
      render: (val: string) => (
        <Tag style={{ fontFamily: 'monospace', fontSize: 10 }}>
          {val.substring(0, 16)}...
        </Tag>
      )
    },
    { title: 'Tarih', dataIndex: 'created_at', key: 'created_at' },
    {
      title: 'İndir',
      key: 'download',
      render: (_: any, record: Backup) => (
        <Button
          type="primary"
          size="small"
          icon={<DownloadOutlined />}
          onClick={() => handleDownload(selectedDevice!.id, record.id)}
        >
          İndir
        </Button>
      )
    }
  ];

  return (
    <div>
      <Title level={2}>Cihazlar</Title>
      <Card
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalVisible(true)}>
            Yeni Cihaz
          </Button>
        }
      >
        <Table
          dataSource={devices}
          columns={deviceColumns}
          rowKey="id"
          loading={loading}
          pagination={false}
        />
      </Card>

      <Modal
        title="Yeni Cihaz Ekle"
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
      >
        <Form form={form} onFinish={handleCreate} layout="vertical">
          <Form.Item
            name="name"
            label="Cihaz Adı"
            rules={[{ required: true, message: 'Cihaz adı gerekli' }]}
          >
            <Input placeholder="Örn: Laptop, Sunucu" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block>
              Ekle
            </Button>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`${selectedDevice?.name} - Yedekler`}
        open={backupModalVisible}
        onCancel={() => setBackupModalVisible(false)}
        footer={null}
        width={800}
      >
        <Table
          dataSource={backups}
          columns={backupColumns}
          rowKey="id"
          loading={backupsLoading}
          pagination={false}
          size="small"
        />
      </Modal>
    </div>
  );
}
