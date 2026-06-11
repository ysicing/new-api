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
import {
  Button,
  Card,
  DatePicker,
  Form,
  Popconfirm,
  Popover,
  Progress,
  Space,
  Spin,
  TabPane,
  Table,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { renderQuota } from '../../helpers/render';
import { useTopUsersData } from '../../hooks/stats/useTopUsersData';
import { useRechargeLeaderboardData } from '../../hooks/stats/useRechargeLeaderboardData';

const { Paragraph } = Typography;

const Stats = () => {
  const { t } = useTranslation();
  const {
    loading,
    topUsers,
    limit,
    setStartTimestamp,
    setEndTimestamp,
    setLimit,
    fetchTopUsers,
  } = useTopUsersData();
  const {
    loading: rechargeLoading,
    leaderboard,
    weeklyLimit,
    limit: rechargeLimit,
    setLimit: setRechargeLimit,
    fetchLeaderboard,
    rechargeUser,
  } = useRechargeLeaderboardData();
  const [dateRange, setDateRange] = useState([]);

  useEffect(() => {
    const now = new Date();
    const todayStart = new Date(
      now.getFullYear(),
      now.getMonth(),
      now.getDate(),
      0,
      0,
      0,
    );
    setStartTimestamp(Math.floor(todayStart.getTime() / 1000));
    setEndTimestamp(Math.floor(now.getTime() / 1000));
    setDateRange([todayStart, now]);
    fetchLeaderboard();
  }, [fetchLeaderboard, setEndTimestamp, setStartTimestamp]);

  const handleDateChange = (nextDateRange) => {
    if (!nextDateRange?.[0] || !nextDateRange?.[1]) {
      return;
    }
    const start = Math.floor(nextDateRange[0].getTime() / 1000);
    const end = Math.floor(nextDateRange[1].getTime() / 1000);
    const maxDuration = 30 * 24 * 60 * 60;
    if (end - start > maxDuration) {
      const adjustedStart = end - maxDuration;
      setStartTimestamp(adjustedStart);
      setEndTimestamp(end);
      setDateRange([new Date(adjustedStart * 1000), new Date(end * 1000)]);
      return;
    }
    setStartTimestamp(start);
    setEndTimestamp(end);
    setDateRange(nextDateRange);
  };

  const renderQuotaUsage = (_, record) => {
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

  const renderUsedQuotaWithModels = (_, record) => {
    const usedQuota = parseInt(record.used_quota) || 0;
    const modelQuotaItems = [
      { label: 'GPT', quota: parseInt(record.gpt_quota) || 0 },
      { label: 'Claude', quota: parseInt(record.claude_quota) || 0 },
      { label: 'DeepSeek', quota: parseInt(record.deepseek_quota) || 0 },
      { label: 'Gemini', quota: parseInt(record.gemini_quota) || 0 },
      { label: 'Qwen', quota: parseInt(record.qwen_quota) || 0 },
      { label: '其他', quota: parseInt(record.other_quota) || 0 },
    ];
    const visibleItems = modelQuotaItems.filter((item) => item.quota !== 0);

    const popoverContent = (
      <div className='text-xs p-2'>
        {visibleItems.length > 0 ? (
          visibleItems.map((item) => {
            const percent =
              usedQuota > 0 ? ` (${((item.quota / usedQuota) * 100).toFixed(0)}%)` : '';
            return (
              <Paragraph key={item.label} copyable={{ content: renderQuota(item.quota) }}>
                {t(item.label)}: {renderQuota(item.quota)}
                {percent}
              </Paragraph>
            );
          })
        ) : (
          <Paragraph>{t('暂无模型使用数据')}</Paragraph>
        )}
      </div>
    );

    return (
      <Popover content={popoverContent} position='top'>
        <Tag color='white' shape='circle'>
          <span className='text-xs leading-none'>{renderQuota(usedQuota)}</span>
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
      render: (_, __, index) => index + 1,
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
      width: 220,
      render: renderQuotaUsage,
    },
    {
      title: t('时间范围内使用额度'),
      dataIndex: 'used_quota',
      key: 'used_quota',
      width: 180,
      render: renderUsedQuotaWithModels,
    },
  ];

  const rechargeColumns = [
    {
      title: t('排名'),
      dataIndex: 'rank',
      key: 'rank',
      width: 80,
      render: (_, __, index) => index + 1,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      key: 'username',
      width: 150,
    },
    {
      title: t('剩余额度/总额度'),
      key: 'quota_usage',
      width: 220,
      render: renderQuotaUsage,
    },
    {
      title: t('本周使用额度'),
      dataIndex: 'used_quota',
      key: 'used_quota',
      width: 150,
      render: (quota) => renderQuota(quota),
    },
    {
      title: t('总充值次数'),
      dataIndex: 'total_count',
      key: 'total_count',
      width: 120,
    },
    {
      title: t('自动充值次数'),
      dataIndex: 'auto_recharge_count',
      key: 'auto_recharge_count',
      width: 130,
    },
    {
      title: t('临时额度次数'),
      dataIndex: 'temp_quota_count',
      key: 'temp_quota_count',
      width: 130,
    },
    {
      title: t('操作'),
      key: 'action',
      width: 100,
      render: (_, record) => {
        if (weeklyLimit <= 0 || record.total_count < weeklyLimit) {
          return null;
        }
        return (
          <Popconfirm
            title={t('确定要给该用户充值吗？')}
            onConfirm={() => rechargeUser(record.user_id)}
          >
            <Button type='warning' size='small'>
              {t('充值')}
            </Button>
          </Popconfirm>
        );
      },
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card>
        <Tabs type='line' defaultActiveKey='top_users'>
          <TabPane tab={t('Top用户统计')} itemKey='top_users'>
            <Form layout='horizontal' style={{ marginBottom: 20, marginTop: 16 }}>
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
                scroll={{ x: 'max-content' }}
                rowKey={(record) => record.user_id || record.username}
                empty={t('暂无数据')}
              />
            </Spin>
          </TabPane>

          <TabPane tab={t('本周充值排行榜')} itemKey='recharge_leaderboard'>
            <Form layout='horizontal' style={{ marginBottom: 20, marginTop: 16 }}>
              <Space>
                <Form.Select
                  field='rechargeLimit'
                  label={t('显示数量')}
                  initValue={10}
                  style={{ width: 150 }}
                  value={rechargeLimit}
                  onChange={(value) => setRechargeLimit(value)}
                >
                  <Form.Select.Option value={10}>Top 10</Form.Select.Option>
                  <Form.Select.Option value={20}>Top 20</Form.Select.Option>
                  <Form.Select.Option value={30}>Top 30</Form.Select.Option>
                </Form.Select>
                <Button
                  type='primary'
                  onClick={fetchLeaderboard}
                  loading={rechargeLoading}
                >
                  {t('查询')}
                </Button>
              </Space>
            </Form>

            <Spin spinning={rechargeLoading}>
              <Table
                columns={rechargeColumns}
                dataSource={leaderboard}
                pagination={false}
                rowKey={(record) => record.user_id || record.username}
                empty={t('暂无数据')}
              />
            </Spin>
          </TabPane>
        </Tabs>
      </Card>
    </div>
  );
};

export default Stats;
