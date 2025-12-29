/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { Card, Table, Button, DatePicker, Form, Typography, Space, Spin, Banner, Tag, Popover, Progress } from '@douyinfe/semi-ui';
import { useTopUsersData } from '../../hooks/stats/useTopUsersData';
import { useTranslation } from 'react-i18next';
import { renderQuota } from '../../helpers/render';

const { Title, Paragraph } = Typography;

const Stats = () => {
  const { t } = useTranslation();
  const {
    loading,
    topUsers,
    startTimestamp,
    endTimestamp,
    limit,
    setStartTimestamp,
    setEndTimestamp,
    setLimit,
    fetchTopUsers,
  } = useTopUsersData();

  const [dateRange, setDateRange] = useState([]);

  useEffect(() => {
    // 默认查询今天的数据（从00:00:00到当前时间）
    const now = new Date();
    const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);
    const startTimestampInSeconds = Math.floor(todayStart.getTime() / 1000);
    const endTimestampInSeconds = Math.floor(now.getTime() / 1000);

    setStartTimestamp(startTimestampInSeconds);
    setEndTimestamp(endTimestampInSeconds);
    setDateRange([todayStart, now]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleDateChange = (dateRange) => {
    if (dateRange && dateRange[0] && dateRange[1]) {
      const start = Math.floor(dateRange[0].getTime() / 1000);
      const end = Math.floor(dateRange[1].getTime() / 1000);

      // 限制最多30天
      const maxDuration = 30 * 24 * 60 * 60;
      if (end - start > maxDuration) {
        // 自动调整为最近30天
        setStartTimestamp(end - maxDuration);
        setEndTimestamp(end);
        setDateRange([new Date((end - maxDuration) * 1000), new Date(end * 1000)]);
      } else {
        setStartTimestamp(start);
        setEndTimestamp(end);
        setDateRange(dateRange);
      }
    }
  };

  // 渲染额度使用情况（类似用户管理页面）
  const renderQuotaUsage = (text, record) => {
    const remain = parseInt(record.remaining_quota) || 0;
    const total = parseInt(record.total_quota) || 0;
    const used = total - remain;
    const percent = total > 0 ? (remain / total) * 100 : 0;

    const popoverContent = (
      <div className='text-xs p-2'>
        <Paragraph copyable={{ content: renderQuota(used) }}>
          {t('已用额度')}: {renderQuota(used)}
        </Paragraph>
        <Paragraph copyable={{ content: renderQuota(remain) }}>
          {t('剩余额度')}: {renderQuota(remain)} ({percent.toFixed(0)}%)
        </Paragraph>
        <Paragraph copyable={{ content: renderQuota(total) }}>
          {t('总额度')}: {renderQuota(total)}
        </Paragraph>
      </div>
    );

    return (
      <Popover content={popoverContent} position='top'>
        <Tag color='white' shape='circle'>
          <div className='flex flex-col items-end'>
            <span className='text-xs leading-none'>{`${renderQuota(remain)} / ${renderQuota(total)}`}</span>
            <Progress
              percent={percent}
              aria-label='quota usage'
              format={() => `${percent.toFixed(0)}%`}
              style={{ width: '100%', marginTop: '1px', marginBottom: 0 }}
            />
          </div>
        </Tag>
      </Popover>
    );
  };

  const columns = [
    {
      title: t('排名'),
      dataIndex: 'rank',
      key: 'rank',
      width: 80,
      render: (text, record, index) => index + 1,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      key: 'username',
      width: 180,
    },
    {
      title: t('剩余额度/总额度'),
      key: 'quota_usage',
      width: 200,
      render: (text, record) => renderQuotaUsage(text, record),
    },
    {
      title: t('时间范围内使用额度'),
      dataIndex: 'used_quota',
      key: 'used_quota',
      width: 180,
      render: (quota) => renderQuota(quota),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card>
        <Title heading={3}>{t('Top用户统计')}</Title>

        <Banner
          type='info'
          description={t('统计时间范围最多30天，默认查询今天数据（00:00:00至当前时间）')}
          closeIcon={null}
          style={{ marginTop: 10, marginBottom: 20 }}
        />

        <Form layout='horizontal' style={{ marginBottom: 20 }}>
          <Space>
            <DatePicker
              type='dateTimeRange'
              density='compact'
              placeholder={[t('开始时间'), t('结束时间')]}
              value={dateRange}
              onChange={handleDateChange}
              style={{ width: 350 }}
            />

            <Form.Select
              field='limit'
              label={t('显示数量')}
              initValue={10}
              style={{ width: 150 }}
              value={limit}
              onChange={(value) => setLimit(value)}
            >
              <Form.Select.Option value={10}>Top 10</Form.Select.Option>
              <Form.Select.Option value={20}>Top 20</Form.Select.Option>
              <Form.Select.Option value={30}>Top 30</Form.Select.Option>
            </Form.Select>

            <Button type='primary' onClick={fetchTopUsers} loading={loading}>
              {t('查询')}
            </Button>
          </Space>
        </Form>

        <Spin spinning={loading}>
          <Table
            columns={columns}
            dataSource={topUsers}
            pagination={false}
            rowKey={(record) => record.username}
            empty={t('暂无数据')}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default Stats;
