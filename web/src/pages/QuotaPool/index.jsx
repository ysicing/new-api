import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Dropdown,
  Form,
  Modal,
  Popconfirm,
  Radio,
  RadioGroup,
  Select,
  Space,
  TabPane,
  Table,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconDelete,
  IconArrowLeft,
  IconEdit,
  IconEyeOpened,
  IconMore,
  IconPlus,
  IconRefresh,
  IconUserAdd,
} from '@douyinfe/semi-icons';
import { renderQuota, timestamp2string } from '../../helpers';
import { useQuotaPoolsData } from '../../hooks/quota-pools/useQuotaPoolsData';

const QUOTA_PER_UNIT = 500000;
const TRANSACTION_TYPE_LABELS = {
  initial_fund: '初始入池',
  manual_refill: '临时额度',
  monthly_refill: '月度扩容',
  allocate_auto: '自动分配',
  allocate_manual: '手动分配',
  reclaim_user: '回收用户额度',
  adjust_base_quota: '调整总额度',
};
const ROLE_MEMBER = 0;
const ROLE_POOL_ADMIN_V1 = 1;

const QuotaPool = () => {
  const data = useQuotaPoolsData();
  const {
    t,
    pools,
    selectedPool,
    setSelectedPool,
    members,
    membersPage,
    membersPageSize,
    membersTotal,
    handleMembersPageChange,
    handleMembersPageSizeChange,
    transactions,
    stats,
    statsLoading,
    statsPeriod,
    setStatsPeriod,
    loadStats,
    candidates,
    loadCandidates,
    createPool,
    syncDefaultPool,
    updatePool,
    setPoolEnabled,
    deletePool,
    refillPool,
    addMember,
    moveMember,
    rechargeMember,
    reclaimMember,
    grantAdmin,
    revokeAdmin,
    canUseGlobalApi,
    canConfigurePools,
    canConfigureRechargeRules,
    canManagePoolAdmins,
    canManagePoolMembers,
    canRefillPools,
    quotaPoolAdmin,
  } = data;
  const [showCreate, setShowCreate] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [showRefill, setShowRefill] = useState(false);
  const [showAddMember, setShowAddMember] = useState(false);
  const [moveMemberRecord, setMoveMemberRecord] = useState(null);

  useEffect(() => {
    if (showAddMember) {
      loadCandidates('');
    }
  }, [showAddMember, loadCandidates]);

  const poolOptions = useMemo(
    () =>
      pools
        .filter((pool) => !pool.is_default)
        .map((pool) => ({
          label: pool.name,
          value: pool.id,
        })),
    [pools],
  );

  const targetPoolOptions = useMemo(
    () =>
      pools
        .filter(
          (pool) =>
            pool.id !== selectedPool?.id && (pool.is_default || pool.enabled),
        )
        .map((pool) => ({
          label: pool.is_default ? t('默认额度池') : pool.name,
          value: pool.is_default ? 0 : pool.id,
        })),
    [pools, selectedPool?.id, t],
  );

  const canOperateSelectedPool = selectedPool && !selectedPool.is_default;
  const canUseActivePool = canOperateSelectedPool && selectedPool.enabled;
  const canMoveMembers =
    canUseGlobalApi && canManagePoolMembers && canOperateSelectedPool;
  const canRechargeMembers = canUseActivePool && canManagePoolMembers;
  const canReclaimMembers = canUseActivePool && canManagePoolMembers;
  const canAddMembers =
    canUseActivePool && (canManagePoolMembers || canManagePoolAdmins);
  const canConfigurePoolRules = (pool) =>
    !!pool &&
    !pool.is_default &&
    (canConfigurePools ||
      (canUseGlobalApi && canConfigureRechargeRules) ||
      (canConfigureRechargeRules && quotaPoolAdmin?.pool_id === pool.id));
  const canConfigureSelectedPool = canConfigurePoolRules(selectedPool);
  const hasDefaultPool = pools.some((pool) => pool.is_default);

  const getMemberRoleActions = (record) => {
    if (!canManagePoolAdmins) {
      return [];
    }
    const currentLevel = record.quota_pool_admin_level || ROLE_MEMBER;
    if (currentLevel === ROLE_MEMBER) {
      return [{ label: t('设为管理员'), level: ROLE_POOL_ADMIN_V1 }];
    }
    return [{ label: t('降为成员'), level: ROLE_MEMBER, danger: true }];
  };

  const updateMemberRole = async (userId, level) => {
    if (level === ROLE_MEMBER) {
      await revokeAdmin(userId);
      return;
    }
    await grantAdmin(userId, level);
  };

  const confirmUpdateMemberRole = (record, level) => {
    Modal.confirm({
      title: t('确定要调整该用户的额度池角色吗？'),
      onOk: () => updateMemberRole(record.id, level),
    });
  };

  const renderPoolStatus = (pool) => {
    if (pool.is_default) {
      return <Tag color='blue'>{t('默认')}</Tag>;
    }
    return (
      <Tag color={pool.enabled ? 'green' : 'red'}>
        {pool.enabled ? t('已启用') : t('已禁用')}
      </Tag>
    );
  };

  const renderPoolQuota = (pool) => {
    if (pool.is_default) {
      return `${t('不限额')} / ${t('不限额')}`;
    }
    return `${renderQuota(pool.quota)} / ${renderQuota(pool.base_quota)}`;
  };

  const renderInheritedValue = (value) => (
    <span className='inline-flex flex-wrap items-baseline gap-1'>
      <span>{value}</span>
      <span className='text-xs text-slate-500'>（{t('全局配置')}）</span>
    </span>
  );

  const renderSystemConfigValue = (value) => (
    <span className='inline-flex flex-wrap items-baseline gap-1'>
      <span>{value}</span>
      <span className='text-xs text-slate-500'>（{t('全局配置')}）</span>
    </span>
  );

  const renderRechargeRule = (pool) => {
    if (pool.is_default) return t('系统默认');
    if (pool.auto_recharge_amount < 0) {
      const systemConfig = pool.system_auto_recharge || {};
      const systemValue =
        systemConfig.enabled && systemConfig.amount > 0
          ? renderQuota(systemConfig.amount)
          : t('关闭');
      return renderInheritedValue(systemValue);
    }
    if (pool.auto_recharge_amount === 0) return t('关闭');
    return renderQuota(pool.auto_recharge_amount);
  };

  const renderLimit = (value, systemValue) => {
    if (value < 0) {
      const inheritedValue =
        typeof systemValue === 'number'
          ? systemValue === 0
            ? t('不限')
            : systemValue
          : t('全局配置');
      return renderInheritedValue(inheritedValue);
    }
    if (value === 0) return t('不限');
    return value;
  };

  const isAutoRechargeClosed = (pool) => {
    const systemConfig = pool?.system_auto_recharge || {};
    if (!pool || !systemConfig.enabled) return true;
    if (pool.auto_recharge_amount === 0) return true;
    return pool.auto_recharge_amount < 0 && (systemConfig.amount || 0) <= 0;
  };

  const renderAutoRechargeRuleSummary = (pool) => {
    const systemConfig = pool?.system_auto_recharge || {};
    if (pool?.is_default) {
      return (
        <Typography.Text type='secondary'>
          {t('全局自动充值规则')}
        </Typography.Text>
      );
    }
    if (!systemConfig.enabled) {
      return (
        <Typography.Text type='secondary'>
          {t('系统自动充值已关闭。')}
        </Typography.Text>
      );
    }
    if (pool.auto_recharge_amount === 0) {
      return (
        <Typography.Text type='secondary'>
          {t('该额度池已关闭自动充值。')}
        </Typography.Text>
      );
    }

    const interval = systemConfig.interval > 0 ? systemConfig.interval : 30;
    const threshold =
      typeof systemConfig.threshold === 'number'
        ? renderQuota(systemConfig.threshold)
        : '-';
    return (
      <span className='inline-flex flex-wrap items-baseline gap-x-1 gap-y-0.5'>
        <span>{t('每 {{interval}} 分钟检查', { interval })}</span>
        <span>{t('低于')}</span>
        {renderSystemConfigValue(threshold)}
        <span>{t('触发')}</span>
      </span>
    );
  };

  const renderTransactionType = (type) =>
    t(TRANSACTION_TYPE_LABELS[type] || type);

  const renderTransactionUser = (id, name) => {
    if (!id || !name) {
      return t('系统');
    }
    return `${name}(ID:${id})`;
  };

  const renderQuotaPoolRole = (level) => {
    if (level > ROLE_MEMBER) return t('池管理员');
    return t('成员');
  };

  const renderModelQuotaPercents = (record, usedQuota, emptyContent = '-') => {
    const items = [
      { label: 'GPT', quota: parseInt(record.gpt_quota) || 0 },
      { label: 'Claude', quota: parseInt(record.claude_quota) || 0 },
      { label: 'DeepSeek', quota: parseInt(record.deepseek_quota) || 0 },
      { label: 'Gemini', quota: parseInt(record.gemini_quota) || 0 },
      { label: 'Qwen', quota: parseInt(record.qwen_quota) || 0 },
      { label: '其他', quota: parseInt(record.other_quota) || 0 },
    ]
      .map((item) => ({
        ...item,
        percent: usedQuota > 0 ? Math.round((item.quota / usedQuota) * 100) : 0,
      }))
      .filter((item) => item.quota !== 0 && item.percent > 0)
      .sort((a, b) => b.quota - a.quota);

    if (items.length === 0) {
      return emptyContent;
    }
    return (
      <div className='flex min-w-0 flex-wrap gap-1'>
        {items.map((item) => (
          <span
            key={item.label}
            className='rounded border border-slate-200 bg-slate-50 px-1.5 py-0.5 text-xs leading-4 text-slate-700'
          >
            {t(item.label)} {item.percent}%
          </span>
        ))}
      </div>
    );
  };

  const adminRoleOptions = [
    ...(canManagePoolMembers ? [{ label: t('成员'), value: ROLE_MEMBER }] : []),
    { label: t('池管理员'), value: ROLE_POOL_ADMIN_V1 },
  ];

  const canDeletePool = (pool) =>
    canConfigurePools &&
    pool &&
    !pool.is_default &&
    (pool.member_count || 0) === 0;

  const openPoolDetail = (pool) => {
    if (pool?.is_default) {
      return;
    }
    setSelectedPool(pool);
  };

  const closePoolDetail = () => {
    setSelectedPool(null);
  };

  const poolColumns = [
    {
      title: t('名称'),
      dataIndex: 'name',
      render: (_, record) => (
        <Space spacing={8}>
          <Typography.Text strong>{record.name}</Typography.Text>
          {renderPoolStatus(record)}
        </Space>
      ),
    },
    {
      title: t('可用额度/总额度'),
      dataIndex: 'quota',
      width: 170,
      render: (_, record) => renderPoolQuota(record),
    },
    {
      title: t('成员数'),
      dataIndex: 'member_count',
      width: 90,
      render: (value) => value || 0,
    },
    {
      title: t('管理员数'),
      dataIndex: 'admin_count',
      width: 100,
      render: (value) => value || 0,
    },
    {
      title: t('充值规则'),
      dataIndex: 'auto_recharge_amount',
      width: 150,
      render: (_, record) => renderRechargeRule(record),
    },
    {
      title: t('周自动充值次数'),
      dataIndex: 'weekly_limit',
      width: 140,
      render: (value, record) =>
        isAutoRechargeClosed(record)
          ? '-'
          : renderLimit(value, record.system_auto_recharge?.weekly_limit),
    },
    {
      title: t('月自动充值次数'),
      dataIndex: 'monthly_limit',
      width: 140,
      render: (value, record) =>
        isAutoRechargeClosed(record)
          ? '-'
          : renderLimit(value, record.system_auto_recharge?.monthly_limit),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      width: 170,
      render: (value) => timestamp2string(value),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      width: 260,
      fixed: 'right',
      render: (_, record) => (
        <Space spacing={6} wrap>
          {!record.is_default && (
            <Button
              size='small'
              icon={<IconEyeOpened />}
              onClick={() => openPoolDetail(record)}
            >
              {t('查看')}
            </Button>
          )}
          {canConfigurePoolRules(record) && (
            <Button
              size='small'
              icon={<IconEdit />}
              onClick={() => {
                setSelectedPool(record);
                setShowConfig(true);
              }}
            >
              {t('配置')}
            </Button>
          )}
          {canConfigurePools && !record.is_default && (
            <Popconfirm
              title={
                record.enabled
                  ? t('确定要禁用该额度池吗？')
                  : t('确定要启用该额度池吗？')
              }
              onConfirm={() => setPoolEnabled(record.id, !record.enabled)}
            >
              <Button
                size='small'
                type={record.enabled ? 'warning' : 'primary'}
              >
                {record.enabled ? t('禁用') : t('启用')}
              </Button>
            </Popconfirm>
          )}
          {canDeletePool(record) && (
            <Popconfirm
              title={t('确定要删除该额度池吗？')}
              onConfirm={() => deletePool(record.id)}
            >
              <Button size='small' type='danger' icon={<IconDelete />}>
                {t('删除')}
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

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
      title: t('角色'),
      dataIndex: 'quota_pool_admin_level',
      width: 110,
      render: (value) =>
        value > 0 ? (
          <Tag color='blue'>{renderQuotaPoolRole(value)}</Tag>
        ) : (
          <Tag color='grey'>{t('成员')}</Tag>
        ),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      width: 150,
      fixed: 'right',
      render: (_, record) => {
        const dropdownItems = [];
        if (canReclaimMembers) {
          dropdownItems.push(
            <Dropdown.Item
              key='reclaim'
              type='danger'
              onClick={() => reclaimMember(record.id)}
            >
              {t('降额')}
            </Dropdown.Item>,
          );
        }
        if (canMoveMembers) {
          dropdownItems.push(
            <Dropdown.Item
              key='move'
              onClick={() => setMoveMemberRecord(record)}
            >
              {t('迁移')}
            </Dropdown.Item>,
          );
        }
        getMemberRoleActions(record).forEach((action) => {
          dropdownItems.push(
            <Dropdown.Item
              key={`role-${record.id}-${action.level}`}
              type={action.danger ? 'danger' : undefined}
              onClick={() => confirmUpdateMemberRole(record, action.level)}
            >
              {action.label}
            </Dropdown.Item>,
          );
        });
        return (
          <Space spacing={6}>
            {canRechargeMembers && (
              <Button size='small' onClick={() => rechargeMember(record.id)}>
                {t('充值')}
              </Button>
            )}
            {dropdownItems.length > 0 && (
              <Dropdown
                trigger='click'
                position='bottomRight'
                render={<Dropdown.Menu>{dropdownItems}</Dropdown.Menu>}
              >
                <Button size='small' type='tertiary' icon={<IconMore />} />
              </Dropdown>
            )}
          </Space>
        );
      },
    },
  ];

  const transactionColumns = [
    {
      title: t('类型'),
      dataIndex: 'type',
      width: 120,
      render: renderTransactionType,
    },
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
    {
      title: t('用户'),
      dataIndex: 'user_id',
      width: 150,
      render: (value, record) => renderTransactionUser(value, record.user_name),
    },
    {
      title: t('操作人'),
      dataIndex: 'operator_id',
      width: 150,
      render: (value, record) =>
        renderTransactionUser(value, record.operator_name),
    },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: (value) => timestamp2string(value),
    },
  ];

  const usageStatColumns = [
    { title: t('ID'), dataIndex: 'user_id', width: 80 },
    { title: t('用户名'), dataIndex: 'username', width: 180 },
    {
      title: t('使用额度'),
      dataIndex: 'used_quota',
      width: 160,
      render: (value) => renderQuota(value),
    },
    {
      title: t('模型占比'),
      dataIndex: 'model_quota_ratio',
      render: (_, record) =>
        renderModelQuotaPercents(record, parseInt(record.used_quota) || 0),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <div className='flex flex-col gap-3'>
        {!selectedPool ? (
          <Card>
            <div className='flex flex-col gap-3'>
              <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
                <div>
                  <Typography.Title heading={4} style={{ margin: 0 }}>
                    {t('额度池')}
                  </Typography.Title>
                  <Typography.Text type='secondary'>
                    {t('管理额度池、池成员和池余额规则')}
                  </Typography.Text>
                </div>
                {canConfigurePools && (
                  <Space>
                    {!hasDefaultPool && (
                      <Button icon={<IconRefresh />} onClick={syncDefaultPool}>
                        {t('同步系统池')}
                      </Button>
                    )}
                    <Button
                      type='primary'
                      icon={<IconPlus />}
                      onClick={() => setShowCreate(true)}
                    >
                      {t('新建额度池')}
                    </Button>
                  </Space>
                )}
              </div>
              <Table
                size='small'
                columns={poolColumns}
                dataSource={pools}
                rowKey='id'
                loading={data.loading}
                pagination={false}
              />
            </div>
          </Card>
        ) : (
          <>
            <Card>
              <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
                <Space>
                  <Button
                    icon={<IconArrowLeft />}
                    type='tertiary'
                    onClick={closePoolDetail}
                  >
                    {t('返回列表')}
                  </Button>
                  <Typography.Text strong>{t('额度池')}</Typography.Text>
                  <Select
                    style={{ width: 240 }}
                    value={selectedPool?.id}
                    optionList={poolOptions}
                    onChange={(id) =>
                      setSelectedPool(pools.find((pool) => pool.id === id))
                    }
                  />
                  {renderPoolStatus(selectedPool)}
                </Space>
                <Space>
                  {canConfigureSelectedPool && (
                    <Button onClick={() => setShowConfig(true)}>
                      {t('配置')}
                    </Button>
                  )}
                  {canConfigurePools && canOperateSelectedPool && (
                    <Popconfirm
                      title={
                        selectedPool.enabled
                          ? t('确定要禁用该额度池吗？')
                          : t('确定要启用该额度池吗？')
                      }
                      onConfirm={() =>
                        setPoolEnabled(selectedPool.id, !selectedPool.enabled)
                      }
                    >
                      <Button
                        type={selectedPool.enabled ? 'warning' : 'primary'}
                      >
                        {selectedPool.enabled ? t('禁用') : t('启用')}
                      </Button>
                    </Popconfirm>
                  )}
                  {canRefillPools && canUseActivePool && (
                    <Button
                      icon={<IconRefresh />}
                      onClick={() => setShowRefill(true)}
                    >
                      {t('临时额度')}
                    </Button>
                  )}
                  {canAddMembers && (
                    <Button
                      icon={<IconUserAdd />}
                      onClick={() => setShowAddMember(true)}
                    >
                      {canManagePoolMembers ? t('添加成员') : t('设置管理员')}
                    </Button>
                  )}
                  {canDeletePool(selectedPool) && (
                    <Popconfirm
                      title={t('确定要删除该额度池吗？')}
                      onConfirm={() => deletePool(selectedPool.id)}
                    >
                      <Button type='danger' icon={<IconDelete />}>
                        {t('删除')}
                      </Button>
                    </Popconfirm>
                  )}
                </Space>
              </div>
            </Card>

            <div
              className={`grid grid-cols-1 md:grid-cols-2 ${
                isAutoRechargeClosed(selectedPool)
                  ? 'xl:grid-cols-3'
                  : 'xl:grid-cols-5'
              } gap-3`}
            >
              <Card title={t('可用额度/总额度')}>
                <Typography.Text>
                  {renderPoolQuota(selectedPool)}
                </Typography.Text>
              </Card>
              <Card title={t('充值金额')}>
                <Typography.Text>
                  {renderRechargeRule(selectedPool)}
                </Typography.Text>
              </Card>
              {!isAutoRechargeClosed(selectedPool) && (
                <>
                  <Card title={t('周自动充值次数')}>
                    <Typography.Text>
                      {renderLimit(
                        selectedPool.weekly_limit,
                        selectedPool.system_auto_recharge?.weekly_limit,
                      )}
                    </Typography.Text>
                  </Card>
                  <Card title={t('月自动充值次数')}>
                    <Typography.Text>
                      {renderLimit(
                        selectedPool.monthly_limit,
                        selectedPool.system_auto_recharge?.monthly_limit,
                      )}
                    </Typography.Text>
                  </Card>
                </>
              )}
              <Card title={t('充值说明')}>
                <Typography.Text>
                  {renderAutoRechargeRuleSummary(selectedPool)}
                </Typography.Text>
              </Card>
            </div>

            <Card>
              <Tabs type='line' defaultActiveKey='members'>
                <TabPane
                  itemKey='members'
                  tab={
                    <span className='flex items-center gap-2'>
                      {t('成员')}
                      <Tag color='grey' shape='circle'>
                        {membersTotal}
                      </Tag>
                    </span>
                  }
                >
                  <Table
                    size='small'
                    columns={memberColumns}
                    dataSource={members}
                    rowKey='id'
                    scroll={{ x: 'max-content' }}
                    pagination={{
                      currentPage: membersPage,
                      pageSize: membersPageSize,
                      total: membersTotal,
                      showSizeChanger: true,
                      pageSizeOptions: [10, 20, 50, 100],
                      onPageChange: handleMembersPageChange,
                      onPageSizeChange: handleMembersPageSizeChange,
                    }}
                  />
                </TabPane>
                <TabPane itemKey='transactions' tab={t('流水')}>
                  <Table
                    size='small'
                    columns={transactionColumns}
                    dataSource={transactions}
                    rowKey='id'
                    scroll={{ x: 'max-content' }}
                    pagination={false}
                  />
                </TabPane>
                <TabPane itemKey='stats' tab={t('数据统计')}>
                  <div className='flex flex-col gap-3 pt-3'>
                    <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
                      <RadioGroup
                        type='button'
                        value={statsPeriod}
                        onChange={(event) => setStatsPeriod(event.target.value)}
                      >
                        <Radio value='week'>{t('本周')}</Radio>
                        <Radio value='month'>{t('本月')}</Radio>
                      </RadioGroup>
                      <Button
                        icon={<IconRefresh />}
                        loading={statsLoading}
                        onClick={loadStats}
                      >
                        {t('刷新统计')}
                      </Button>
                    </div>
                    <div className='grid grid-cols-1 md:grid-cols-3 gap-3'>
                      <Card title={t('使用统计')}>
                        <Typography.Text strong>
                          {renderQuota(stats.total_usage || 0)}
                        </Typography.Text>
                      </Card>
                    </div>
                    <Card title={t('使用统计')}>
                      <Table
                        size='small'
                        columns={usageStatColumns}
                        dataSource={stats.usage || []}
                        rowKey='user_id'
                        loading={statsLoading}
                        pagination={false}
                        scroll={{ x: 'max-content' }}
                      />
                    </Card>
                  </div>
                </TabPane>
              </Tabs>
            </Card>
          </>
        )}
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
          <Form.Input
            field='name'
            label={t('名称')}
            rules={[{ required: true }]}
          />
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
            base_quota:
              selectedPool?.base_quota > 0
                ? selectedPool.base_quota / QUOTA_PER_UNIT
                : 0,
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
            const submitValues = canConfigurePools
              ? values
              : {
                  auto_recharge_amount: values.auto_recharge_amount,
                  weekly_limit: values.weekly_limit,
                  monthly_limit: values.monthly_limit,
                };
            await updatePool(selectedPool.id, submitValues);
            setShowConfig(false);
          }}
        >
          {canConfigurePools && <Form.Input field='name' label={t('名称')} />}
          {canConfigurePools && (
            <Form.InputNumber
              field='base_quota'
              label={t('总额度')}
              min={1}
              rules={[{ required: true }]}
              extraText={t(
                '调整总额度会按差额同步调整额度池可用额度；下调后可用额度不能小于 0。',
              )}
            />
          )}
          <Form.InputNumber
            field='auto_recharge_amount'
            label={t('充值金额')}
            extraText={t(
              '特殊值：-1 继承全局配置，0 关闭自动充值，正数为自定义充值金额。',
            )}
          />
          <Form.InputNumber
            field='weekly_limit'
            label={t('周自动充值次数')}
            extraText={t(
              '特殊值：-1 继承全局配置，0 不限制次数，正数为周期内最多充值次数。',
            )}
          />
          <Form.InputNumber
            field='monthly_limit'
            label={t('月自动充值次数')}
            extraText={t(
              '特殊值：-1 继承全局配置，0 不限制次数，正数为周期内最多充值次数。',
            )}
          />
          {canConfigurePools && (
            <>
              <Form.Switch
                field='monthly_refill_enabled'
                label={t('月度扩容')}
              />
              <Form.InputNumber
                field='monthly_refill_amount'
                label={t('月扩容金额')}
              />
              <Form.InputNumber
                field='monthly_refill_day'
                label={t('扩容日期')}
                min={1}
                max={28}
              />
            </>
          )}
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
          <Form.InputNumber
            field='amount'
            label={t('金额')}
            min={1}
            rules={[{ required: true }]}
          />
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
            if (canManagePoolMembers) {
              await addMember(values.user_id);
            }
            if (values.admin_level > 0) {
              await grantAdmin(values.user_id, values.admin_level);
            }
            setShowAddMember(false);
          }}
        >
          <Form.Select
            field='user_id'
            label={t('用户')}
            placeholder={t('搜索用户 ID、用户名、显示名称或邮箱')}
            style={{ width: '100%' }}
            optionList={candidates.map((user) => ({
              label: `${user.username}(ID:${user.id})`,
              value: user.id,
            }))}
            filter
            remote
            showClear
            onSearch={(keyword) => loadCandidates(keyword)}
            rules={[{ required: true }]}
          />
          {canManagePoolAdmins && (
            <Form.Select
              field='admin_level'
              label={t('角色')}
              extraText={t(
                '成员仅消耗本池额度；池管理员 v1 可查看本池、添加默认池用户并给成员充值；池超级管理员是系统角色，可为非系统池设置或撤销池管理员。',
              )}
              style={{ width: '100%' }}
              optionList={adminRoleOptions}
              initValue={
                canManagePoolMembers ? ROLE_MEMBER : ROLE_POOL_ADMIN_V1
              }
            />
          )}
          <Button htmlType='submit'>{t('提交')}</Button>
        </Form>
      </Modal>

      <Modal
        title={t('迁移成员')}
        visible={!!moveMemberRecord}
        onCancel={() => setMoveMemberRecord(null)}
        footer={null}
      >
        <Form
          onSubmit={async (values) => {
            await moveMember(moveMemberRecord.id, values.pool_id);
            setMoveMemberRecord(null);
          }}
        >
          <Form.Slot label={t('用户')}>
            <Typography.Text>
              {moveMemberRecord?.id} {moveMemberRecord?.username}
            </Typography.Text>
          </Form.Slot>
          <Form.Select
            field='pool_id'
            label={t('目标额度池')}
            optionList={targetPoolOptions}
            rules={[{ required: true }]}
          />
          <Typography.Paragraph type='secondary' spacing='extended'>
            {t('迁移会清零用户当前额度，并按规则退回原池余额。')}
          </Typography.Paragraph>
          <Button htmlType='submit'>{t('提交')}</Button>
        </Form>
      </Modal>
    </div>
  );
};

export default QuotaPool;
