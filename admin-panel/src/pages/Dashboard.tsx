import { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Progress, Table, Typography } from 'antd';
import {
  CloudServerOutlined,
  HddOutlined,
  UserOutlined,
  DatabaseOutlined
} from '@ant-design/icons';
import { getQuota, getUsage } from '../api';

const { Title } = Typography;

interface QuotaData {
  plan_gb: number;
  used_mb: number;
  used_gb: number;
  free_gb: number;
  used_percentage: number;
}

interface DeviceUsage {
  device_id: number;
  device_name: string;
  backup_count: number;
  size_mb: number;
}

interface UsageData {
  total_backups: number;
  total_devices: number;
  total_size_mb: number;
  device_usage: DeviceUsage[];
}

export default function Dashboard() {
  const [quota, setQuota] = useState<QuotaData | null>(null);
  const [usage, setUsage] = useState<UsageData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [quotaRes, usageRes] = await Promise.all([getQuota(), getUsage()]);
        setQuota(quotaRes.data.data);
        setUsage(usageRes.data.data);
      } catch (error) {
        console.error('Failed to fetch dashboard data:', error);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const columns = [
    { title: 'Cihaz', dataIndex: 'device_name', key: 'device_name' },
    { title: 'Yedek Sayısı', dataIndex: 'backup_count', key: 'backup_count' },
    {
      title: 'Boyut',
      dataIndex: 'size_mb',
      key: 'size_mb',
      render: (val: number) => `${val.toFixed(2)} MB`
    }
  ];

  return (
    <div>
      <Title level={2}>Dashboard</Title>

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card loading={loading}>
            <Statistic
              title="Toplam Cihaz"
              value={usage?.total_devices || 0}
              prefix={<HddOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card loading={loading}>
            <Statistic
              title="Toplam Yedek"
              value={usage?.total_backups || 0}
              prefix={<DatabaseOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card loading={loading}>
            <Statistic
              title="Kullanılan Alan"
              value={usage?.total_size_mb?.toFixed(2) || 0}
              suffix="MB"
              prefix={<CloudServerOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card loading={loading}>
            <Statistic
              title="Plan"
              value={quota?.plan_gb || 0}
              suffix="GB"
              prefix={<UserOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="Kota Kullanımı" loading={loading}>
            <Progress
              type="dashboard"
              percent={Number((quota?.used_percentage || 0).toFixed(1))}
              format={() => `${quota?.used_gb?.toFixed(2) || 0} / ${quota?.plan_gb || 0} GB`}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="Cihaz Kullanımı" loading={loading}>
            <Table
              dataSource={usage?.device_usage || []}
              columns={columns}
              rowKey="device_id"
              pagination={false}
              size="small"
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
