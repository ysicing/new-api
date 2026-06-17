import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconRefresh, IconUserAdd } from '@douyinfe/semi-icons';
import { renderQuota, timestamp2string } from '../../helpers';
import { useQuotaPoolsData } from '../../hooks/quota-pools/useQuotaPoolsData';

const QUOTA_PER_UNIT = 500000;

const QuotaPool = () => {
  const data = useQuotaPoolsData();
  const {
    t,
    pools,
    selectedPool,
    setSelectedPool,
    members,
    transactions,
    candidates,
    loadCandidates,
    createPool,
    updatePool,
    refillPool,
    addMember,
    rechargeMember,
    grantAdmin,
    canConfigurePools,
    canManagePoolAdmins,
    canRefillPools,
  } = data;
  const [showCreate, setShowCreate] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [showRefill, setShowRefill] = useState(false);
  const [showAddMember, setShowAddMember] = useState(false);

  useEffect(() => {
    if (showAddMember) {
      loadCandidates('');
    }
  }, [showAddMember, loadCandidates]);

  const poolOptions = useMemo(
    () =>
      pools.map((pool) => ({
        label: pool.name,
        value: pool.id,
      })),
    [pools],
  );

  const memberColumns = [
    { title: t('ID'), dataIndex: 'id', width: 80 },
    { title: t('用户名'), dataIndex: 'username' },
    {
      title: t('余额'),
      dataIndex: 'quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => (
        <Tag color={value === 1 ? 'green' : 'red'}>
          {value === 1 ? t('已启用') : t('已禁用')}
        </Tag>
      ),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      width: 120,
      render: (_, record) => (
        <Button size='small' onClick={() => rechargeMember(record.id)}>
          {t('充值')}
        </Button>
      ),
    },
  ];

  const transactionColumns = [
    { title: t('类型'), dataIndex: 'type', width: 150 },
    {
      title: t('金额'),
      dataIndex: 'amount',
      render: (value) => renderQuota(Math.abs(value)),
    },
    {
      title: t('变更前'),
      dataIndex: 'quota_before',
      render: (value) => renderQuota(value),
    },
    {
      title: t('变更后'),
      dataIndex: 'quota_after',
      render: (value) => renderQuota(value),
    },
    { title: t('用户'), dataIndex: 'user_id', width: 90 },
    { title: t('操作人'), dataIndex: 'operator_id', width: 90 },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: (value) => timestamp2string(value),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <div className='flex flex-col gap-3'>
        <Card>
          <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
            <Space>
              <Typography.Text strong>{t('额度池')}</Typography.Text>
              <Select
                style={{ width: 240 }}
                value={selectedPool?.id}
                optionList={poolOptions}
                onChange={(id) =>
                  setSelectedPool(pools.find((pool) => pool.id === id))
                }
              />
              {selectedPool && (
                <Tag color={selectedPool.enabled ? 'green' : 'red'}>
                  {selectedPool.enabled ? t('已启用') : t('已禁用')}
                </Tag>
              )}
            </Space>
            <Space>
              {canConfigurePools && (
                <Button icon={<IconPlus />} onClick={() => setShowCreate(true)}>
                  {t('新建')}
                </Button>
              )}
              {canConfigurePools && selectedPool && !selectedPool.is_default && (
                <Button onClick={() => setShowConfig(true)}>
                  {t('配置')}
                </Button>
              )}
              {canRefillPools && selectedPool && !selectedPool.is_default && (
                <Button icon={<IconRefresh />} onClick={() => setShowRefill(true)}>
                  {t('临时额度')}
                </Button>
              )}
              {selectedPool && !selectedPool.is_default && (
                <Button icon={<IconUserAdd />} onClick={() => setShowAddMember(true)}>
                  {t('添加成员')}
                </Button>
              )}
            </Space>
          </div>
        </Card>

        {selectedPool && (
          <div className='grid grid-cols-1 md:grid-cols-4 gap-3'>
            <Card title={t('池余额')}>
              <Typography.Text>
                {selectedPool.is_default
                  ? t('不限额')
                  : renderQuota(selectedPool.quota)}
              </Typography.Text>
            </Card>
            <Card title={t('充值金额')}>
              <Typography.Text>
                {selectedPool.auto_recharge_amount < 0
                  ? t('继承系统配置')
                  : selectedPool.auto_recharge_amount === 0
                    ? t('关闭')
                    : renderQuota(selectedPool.auto_recharge_amount)}
              </Typography.Text>
            </Card>
            <Card title={t('周次数')}>
              <Typography.Text>
                {selectedPool.weekly_limit < 0
                  ? t('继承')
                  : selectedPool.weekly_limit === 0
                    ? t('不限')
                    : selectedPool.weekly_limit}
              </Typography.Text>
            </Card>
            <Card title={t('月次数')}>
              <Typography.Text>
                {selectedPool.monthly_limit < 0
                  ? t('继承')
                  : selectedPool.monthly_limit === 0
                    ? t('不限')
                    : selectedPool.monthly_limit}
              </Typography.Text>
            </Card>
          </div>
        )}

        <Card title={t('成员')}>
          <Table
            size='small'
            columns={memberColumns}
            dataSource={members}
            rowKey='id'
            pagination={false}
          />
        </Card>

        <Card title={t('流水')}>
          <Table
            size='small'
            columns={transactionColumns}
            dataSource={transactions}
            rowKey='id'
            pagination={false}
          />
        </Card>
      </div>

      <Modal
        title={t('新建额度池')}
        visible={showCreate}
        onCancel={() => setShowCreate(false)}
        footer={null}
      >
        <Form
          onSubmit={async (values) => {
            await createPool(values);
            setShowCreate(false);
          }}
        >
          <Form.Input field='name' label={t('名称')} rules={[{ required: true }]} />
          <Form.InputNumber
            field='base_quota'
            label={t('预付金额')}
            min={1}
            rules={[{ required: true }]}
          />
          <Button htmlType='submit'>{t('提交')}</Button>
        </Form>
      </Modal>

      <Modal
        title={t('配置额度池')}
        visible={showConfig}
        onCancel={() => setShowConfig(false)}
        footer={null}
      >
        <Form
          initValues={{
            name: selectedPool?.name,
            auto_recharge_amount:
              selectedPool?.auto_recharge_amount > 0
                ? selectedPool.auto_recharge_amount / QUOTA_PER_UNIT
                : selectedPool?.auto_recharge_amount,
            weekly_limit: selectedPool?.weekly_limit,
            monthly_limit: selectedPool?.monthly_limit,
            monthly_refill_enabled: selectedPool?.monthly_refill_enabled,
            monthly_refill_amount:
              selectedPool?.monthly_refill_amount > 0
                ? selectedPool.monthly_refill_amount / QUOTA_PER_UNIT
                : 0,
            monthly_refill_day: selectedPool?.monthly_refill_day || 1,
          }}
          onSubmit={async (values) => {
            await updatePool(selectedPool.id, values);
            setShowConfig(false);
          }}
        >
          <Form.Input field='name' label={t('名称')} />
          <Form.InputNumber field='auto_recharge_amount' label={t('充值金额')} />
          <Form.InputNumber field='weekly_limit' label={t('周次数')} />
          <Form.InputNumber field='monthly_limit' label={t('月次数')} />
          <Form.Switch field='monthly_refill_enabled' label={t('月度扩容')} />
          <Form.InputNumber field='monthly_refill_amount' label={t('月扩容金额')} />
          <Form.InputNumber field='monthly_refill_day' label={t('扩容日期')} min={1} max={28} />
          <Button htmlType='submit'>{t('提交')}</Button>
        </Form>
      </Modal>

      <Modal
        title={t('添加临时额度')}
        visible={showRefill}
        onCancel={() => setShowRefill(false)}
        footer={null}
      >
        <Form
          onSubmit={async (values) => {
            await refillPool(selectedPool.id, values.amount);
            setShowRefill(false);
          }}
        >
          <Form.InputNumber field='amount' label={t('金额')} min={1} rules={[{ required: true }]} />
          <Button htmlType='submit'>{t('提交')}</Button>
        </Form>
      </Modal>

      <Modal
        title={t('添加成员')}
        visible={showAddMember}
        onCancel={() => setShowAddMember(false)}
        footer={null}
      >
        <Form
          onSubmit={async (values) => {
            await addMember(values.user_id);
            if (values.admin_level > 0) {
              await grantAdmin(values.user_id, values.admin_level);
            }
            setShowAddMember(false);
          }}
        >
          <Form.Select
            field='user_id'
            label={t('用户')}
            optionList={candidates.map((user) => ({
              label: `${user.id} ${user.username}`,
              value: user.id,
            }))}
            filter
            remote
            onSearch={(keyword) => loadCandidates(keyword)}
            rules={[{ required: true }]}
          />
          {canManagePoolAdmins && (
            <Form.Select
              field='admin_level'
              label={t('管理员等级')}
              optionList={[
                { label: t('不任命'), value: 0 },
                { label: 'v1', value: 1 },
                { label: 'v2', value: 2 },
              ]}
              initValue={0}
            />
          )}
          <Button htmlType='submit'>{t('提交')}</Button>
        </Form>
      </Modal>
    </div>
  );
};

export default QuotaPool;
