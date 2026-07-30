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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  DatePicker,
  Dropdown,
  Form,
  Input,
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
const QUOTA_POOL_TYPE_DEFAULT = 'default';
const QUOTA_POOL_TYPE_NEW_USER = 'new_user';
const TRANSACTION_TYPE_LABELS = {
  initial_fund: '初始入池',
  manual_refill: '临时额度',
  monthly_refill: '月度自动充值',
  allocate_auto: '自动分配',
  allocate_manual: '手动分配',
  reclaim_user: '回收用户额度',
  adjust_base_quota: '调整总额度',
};
const ROLE_MEMBER = 0;
const ROLE_POOL_ADMIN_V1 = 1;
const TRANSACTION_FILTER_OPTIONS = [
  { label: '全部类型', value: '' },
  { label: '手动', value: 'manual' },
  { label: '自动', value: 'auto' },
  { label: '回收', value: 'reclaim' },
];

const formatDepartment = (department) => {
  if (!department) {
    return '';
  }
  const parts = department
    .split(/[\/\\>｜|,，;；-]+/)
    .map((part) => part.trim())
    .filter(Boolean);
  if (parts.length <= 1) {
    return department;
  }
  return `${parts[0]} / ${parts[parts.length - 1]}`;
};

const formatCandidateLabel = (user) => {
  const details = [
    user.display_name,
    user.email,
    formatDepartment(user.department),
  ].filter(Boolean);
  return `${user.username}(ID:${user.id})${
    details.length > 0 ? ` - ${details.join(' / ')}` : ''
  }`;
};

const RECLAIM_PARTIAL_FACTORS = [50, 40, 30, 20, 10];

const getReclaimAmountOptions = (pool, userQuota) => {
  const systemConfig = pool?.system_auto_recharge || {};
  let autoRechargeAmount = 0;
  if (pool?.is_default || pool?.auto_recharge_amount < 0) {
    autoRechargeAmount = systemConfig.amount || 0;
  } else if (pool?.auto_recharge_amount > 0) {
    autoRechargeAmount = pool.auto_recharge_amount;
  }
  const threshold = systemConfig.enabled ? systemConfig.threshold || 0 : -1;
  if (
    autoRechargeAmount <= 0 ||
    typeof userQuota !== 'number' ||
    userQuota <= threshold
  ) {
    return [];
  }
  if (
    userQuota > autoRechargeAmount &&
    userQuota - autoRechargeAmount > threshold
  ) {
    return [{ amount: autoRechargeAmount, isFull: true }];
  }
  const amounts = RECLAIM_PARTIAL_FACTORS.map((factor) =>
    Math.floor((autoRechargeAmount * factor) / 100),
  ).filter(
    (amount, index, allAmounts) =>
      amount > 0 &&
      userQuota - amount > threshold &&
      allAmounts.indexOf(amount) === index,
  );
  return amounts.map((amount) => ({ amount, isFull: false }));
};

const QuotaPool = () => {
  const data = useQuotaPoolsData();
  const {
    t,
    pools,
    defaultPool,
    selectedPool,
    setSelectedPool,
    members,
    membersPage,
    membersPageSize,
    membersTotal,
    handleMembersPageChange,
    handleMembersPageSizeChange,
    transactions,
    transactionsPage,
    transactionsPageSize,
    transactionsTotal,
    operationLogs,
    operationLogsPage,
    operationLogsPageSize,
    operationLogsTotal,
    adminContacts,
    weeklyAutoRechargeUsage,
    transactionFilters,
    operationLogFilters,
    handleTransactionsPageChange,
    handleTransactionsPageSizeChange,
    handleOperationLogsPageChange,
    handleOperationLogsPageSizeChange,
    updateTransactionFilter,
    updateOperationLogFilter,
    searchTransactions,
    searchOperationLogs,
    resetTransactionFilters,
    resetOperationLogFilters,
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
    quotaPoolSuperAdmin,
    canConfigurePools,
    canConfigureRechargeRules,
    canManagePoolAdmins,
    canManagePoolMembers,
    canRefillPools,
    canViewPoolManagement,
    quotaPoolAdmin,
  } = data;
  const [showCreate, setShowCreate] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [showRefill, setShowRefill] = useState(false);
  const [showAddMember, setShowAddMember] = useState(false);
  const [moveMemberRecord, setMoveMemberRecord] = useState(null);
  const [monthlyRefillEnabled, setMonthlyRefillEnabled] = useState(false);

  useEffect(() => {
    if (showAddMember) {
      loadCandidates('');
    }
  }, [showAddMember, loadCandidates]);

  useEffect(() => {
    if (showConfig) {
      setMonthlyRefillEnabled(!!selectedPool?.monthly_refill_enabled);
    }
  }, [selectedPool?.id, selectedPool?.monthly_refill_enabled, showConfig]);

  const isNewUserPool = (pool) => pool?.pool_type === QUOTA_POOL_TYPE_NEW_USER;
  const isProtectedSystemPool = (pool) =>
    pool?.is_default ||
    pool?.pool_type === QUOTA_POOL_TYPE_DEFAULT ||
    isNewUserPool(pool);

  const poolOptions = useMemo(
    () =>
      pools.map((pool) => ({
        label: pool.name || t('默认额度池'),
        value: pool.is_default ? 0 : pool.id,
      })),
    [pools, t],
  );

  const targetPoolOptions = useMemo(() => {
    if (!canUseGlobalApi) {
      return defaultPool
        ? [
            {
              label: defaultPool.name || t('默认额度池'),
              value: defaultPool.id,
            },
          ]
        : [];
    }
    return pools
      .filter(
        (pool) =>
          pool.id !== selectedPool?.id &&
          (quotaPoolSuperAdmin
            ? !pool.is_default && pool.enabled
            : pool.is_default || pool.enabled),
      )
      .map((pool) => ({
        label: pool.name || t('默认额度池'),
        value: pool.is_default ? 0 : pool.id,
      }));
  }, [
    canUseGlobalApi,
    defaultPool,
    pools,
    quotaPoolSuperAdmin,
    selectedPool?.id,
    t,
  ]);

  const canOperateSelectedPool =
    selectedPool && !isProtectedSystemPool(selectedPool);
  const canUseActivePool = canOperateSelectedPool && selectedPool.enabled;
  const canMoveMembers = canManagePoolMembers && canOperateSelectedPool;
  const canRechargeMembers = canUseActivePool && canManagePoolMembers;
  const canReclaimMembers = canUseActivePool && canManagePoolMembers;
  const canAddMembers =
    canUseActivePool && (canManagePoolMembers || canManagePoolAdmins);
  const canConfigurePoolRules = (pool) =>
    !!pool &&
    !isProtectedSystemPool(pool) &&
    (canConfigurePools ||
      (canUseGlobalApi && canConfigureRechargeRules) ||
      (canConfigureRechargeRules && quotaPoolAdmin?.pool_id === pool.id));
  const canConfigureSelectedPool = canConfigurePoolRules(selectedPool);
  const hasDefaultPool = pools.some((pool) => pool.is_default);

  const getMemberRoleActions = (record) => {
    if (!canManagePoolAdmins || !canOperateSelectedPool) {
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
      return <Tag color='blue'>{t('存量')}</Tag>;
    }
    if (isNewUserPool(pool)) {
      return <Tag color='cyan'>{t('默认')}</Tag>;
    }
    return (
      <Tag color={pool.enabled ? 'green' : 'red'}>
        {pool.enabled ? t('已启用') : t('已禁用')}
      </Tag>
    );
  };

  const renderPoolQuota = (pool) => {
    if (pool.is_default || isNewUserPool(pool)) {
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
    if (isNewUserPool(pool)) return t('关闭');
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

  const showMemberWeeklyAutoRechargeUsage =
    !canViewPoolManagement && weeklyAutoRechargeUsage?.enabled;

  const renderAutoRechargeRuleSummary = (pool) => {
    const systemConfig = pool?.system_auto_recharge || {};
    if (isNewUserPool(pool)) {
      return (
        <Typography.Text type='secondary'>
          {t('该额度池不支持自动充值。')}
        </Typography.Text>
      );
    }
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

  const renderAdminContactName = (admin) => {
    const name =
      admin.display_name || admin.username || `${t('用户')} #${admin.id}`;
    return `${name}(ID:${admin.id})`;
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
    !isProtectedSystemPool(pool) &&
    (pool.member_count || 0) === 0;

  const openPoolDetail = (pool) => {
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
      title: t('本月可用额度/累计总额度'),
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
          <Button
            size='small'
            icon={<IconEyeOpened />}
            onClick={() => openPoolDetail(record)}
          >
            {t('查看')}
          </Button>
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
          {canConfigurePools && !isProtectedSystemPool(record) && (
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
      title: t('剩余额度/总额度'),
      dataIndex: 'quota',
      render: (value, record) =>
        `${renderQuota(value)} / ${renderQuota(value + (record.used_quota || 0))}`,
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
        const reclaimAmounts = getReclaimAmountOptions(
          selectedPool,
          record.quota,
        );
        if (canReclaimMembers) {
          reclaimAmounts.forEach((reclaimOption) => {
            dropdownItems.push(
              <Dropdown.Item
                key={`reclaim-${reclaimOption.amount}`}
                type='danger'
                onClick={() => reclaimMember(record.id, reclaimOption.amount)}
              >
                {reclaimOption.isFull
                  ? t('降额')
                  : t('降额 {{amount}}', {
                      amount: renderQuota(reclaimOption.amount),
                    })}
              </Dropdown.Item>,
            );
          });
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

  const operationLogColumns = [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 80,
    },
    {
      title: t('操作内容'),
      dataIndex: 'content',
      width: 240,
    },
    {
      title: t('关联用户'),
      dataIndex: 'user_id',
      width: 150,
      render: (value, record) => renderTransactionUser(value, record.username),
    },
    {
      title: t('操作人'),
      dataIndex: 'admin_id',
      width: 150,
      render: (value, record) =>
        renderTransactionUser(value, record.admin_username),
    },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      width: 170,
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
                  {canViewPoolManagement && (
                    <Button
                      icon={<IconArrowLeft />}
                      type='tertiary'
                      onClick={closePoolDetail}
                    >
                      {t('返回列表')}
                    </Button>
                  )}
                  <Typography.Text strong>{t('额度池')}</Typography.Text>
                  {canViewPoolManagement ? (
                    <Select
                      style={{ width: 240 }}
                      value={selectedPool?.is_default ? 0 : selectedPool?.id}
                      optionList={poolOptions}
                      onChange={(id) =>
                        setSelectedPool(
                          pools.find(
                            (pool) => (pool.is_default ? 0 : pool.id) === id,
                          ),
                        )
                      }
                    />
                  ) : (
                    <Typography.Text>{selectedPool?.name}</Typography.Text>
                  )}
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

            {isNewUserPool(selectedPool) && (
              <Banner
                type='warning'
                description={t(
                  '请尽快联系部门 iCode 额度管理员添加到对应额度池。',
                )}
              />
            )}

            <div
              className={`grid grid-cols-1 md:grid-cols-2 ${
                isAutoRechargeClosed(selectedPool)
                  ? 'xl:grid-cols-3'
                  : canViewPoolManagement || showMemberWeeklyAutoRechargeUsage
                    ? 'xl:grid-cols-5'
                    : 'xl:grid-cols-4'
              } gap-3`}
            >
              <Card title={t('本月可用额度/累计总额度')}>
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
                  {canViewPoolManagement ? (
                    <Card title={t('周自动充值次数')}>
                      <Typography.Text>
                        {renderLimit(
                          selectedPool.weekly_limit,
                          selectedPool.system_auto_recharge?.weekly_limit,
                        )}
                      </Typography.Text>
                    </Card>
                  ) : (
                    showMemberWeeklyAutoRechargeUsage && (
                      <Card title={t('本周自动充值')}>
                        <div className='flex flex-col gap-1' aria-live='polite'>
                          <Typography.Text>
                            {t('已充值次数')}：{weeklyAutoRechargeUsage.used}
                          </Typography.Text>
                          {weeklyAutoRechargeUsage.limit > 0 && (
                            <Typography.Text type='secondary'>
                              {t('理论剩余次数')}：
                              {weeklyAutoRechargeUsage.remaining}
                            </Typography.Text>
                          )}
                        </div>
                      </Card>
                    )
                  )}
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

            {!canViewPoolManagement && (
              <Card title={t('池管理员')}>
                <div className='flex flex-col gap-3'>
                  <Typography.Text type='secondary'>
                    {t('额度不足时，请联系池管理员充值。')}
                  </Typography.Text>
                  {adminContacts.length > 0 ? (
                    <div className='flex flex-wrap gap-2'>
                      {adminContacts.map((admin) => (
                        <div
                          key={admin.id}
                          className='flex flex-col gap-1 rounded border border-semi-color-border px-3 py-2 min-w-[220px]'
                        >
                          <Typography.Text strong>
                            {renderAdminContactName(admin)}
                          </Typography.Text>
                          <Typography.Text
                            size='small'
                            type='tertiary'
                            copyable={!!admin.email}
                          >
                            {admin.email || t('邮箱未设置')}
                          </Typography.Text>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <Typography.Text type='secondary'>
                      {t('暂无池管理员')}
                    </Typography.Text>
                  )}
                </div>
              </Card>
            )}

            {canViewPoolManagement && (
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
                    <div className='flex flex-col gap-2 py-3'>
                      <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-2'>
                        <Input
                          value={transactionFilters.user}
                          placeholder={t('用户名或用户ID')}
                          showClear
                          pure
                          onChange={(value) =>
                            updateTransactionFilter('user', value)
                          }
                        />
                        <Select
                          value={transactionFilters.transactionType}
                          optionList={TRANSACTION_FILTER_OPTIONS.map(
                            (option) => ({
                              ...option,
                              label: t(option.label),
                            }),
                          )}
                          onChange={(value) =>
                            updateTransactionFilter('transactionType', value)
                          }
                          style={{ width: '100%' }}
                        />
                        <div className='md:col-span-2'>
                          <DatePicker
                            type='dateTimeRange'
                            density='compact'
                            value={transactionFilters.dateRange}
                            placeholder={[t('开始时间'), t('结束时间')]}
                            onChange={(value) =>
                              updateTransactionFilter('dateRange', value || [])
                            }
                            style={{ width: '100%' }}
                          />
                        </div>
                      </div>
                      <Space>
                        <Button type='primary' onClick={searchTransactions}>
                          {t('查询')}
                        </Button>
                        <Button onClick={resetTransactionFilters}>
                          {t('重置')}
                        </Button>
                      </Space>
                    </div>
                    <Table
                      size='small'
                      columns={transactionColumns}
                      dataSource={transactions}
                      rowKey='id'
                      scroll={{ x: 'max-content' }}
                      pagination={{
                        currentPage: transactionsPage,
                        pageSize: transactionsPageSize,
                        total: transactionsTotal,
                        showSizeChanger: true,
                        pageSizeOptions: [10, 20, 50, 100],
                        onPageChange: handleTransactionsPageChange,
                        onPageSizeChange: handleTransactionsPageSizeChange,
                      }}
                    />
                  </TabPane>
                  <TabPane itemKey='operation_logs' tab={t('操作日志')}>
                    <div className='flex flex-col gap-2 py-3'>
                      <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-2'>
                        <Input
                          value={operationLogFilters.keyword}
                          placeholder={t('操作内容/用户/操作人')}
                          showClear
                          pure
                          onChange={(value) =>
                            updateOperationLogFilter('keyword', value)
                          }
                        />
                        <div className='md:col-span-2 xl:col-span-3'>
                          <DatePicker
                            type='dateTimeRange'
                            density='compact'
                            value={operationLogFilters.dateRange}
                            placeholder={[t('开始时间'), t('结束时间')]}
                            onChange={(value) =>
                              updateOperationLogFilter('dateRange', value || [])
                            }
                            style={{ width: '100%' }}
                          />
                        </div>
                      </div>
                      <Space>
                        <Button type='primary' onClick={searchOperationLogs}>
                          {t('查询')}
                        </Button>
                        <Button onClick={resetOperationLogFilters}>
                          {t('重置')}
                        </Button>
                      </Space>
                    </div>
                    <Table
                      size='small'
                      columns={operationLogColumns}
                      dataSource={operationLogs}
                      rowKey='id'
                      scroll={{ x: 'max-content' }}
                      pagination={{
                        currentPage: operationLogsPage,
                        pageSize: operationLogsPageSize,
                        total: operationLogsTotal,
                        showSizeChanger: true,
                        pageSizeOptions: [10, 20, 50, 100],
                        onPageChange: handleOperationLogsPageChange,
                        onPageSizeChange: handleOperationLogsPageSizeChange,
                      }}
                    />
                  </TabPane>
                  <TabPane itemKey='stats' tab={t('数据统计')}>
                    <div className='flex flex-col gap-3 pt-3'>
                      <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
                        <RadioGroup
                          type='button'
                          value={statsPeriod}
                          onChange={(event) =>
                            setStatsPeriod(event.target.value)
                          }
                        >
                          <Radio value='week'>{t('本周')}</Radio>
                          <Radio value='month'>{t('近一月')}</Radio>
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
            )}
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
            monthly_refill_top_up: selectedPool?.monthly_refill_top_up || false,
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
              '特殊值：-1 继承全局配置，0 关闭自动充值；正数需大于触发充值金额，当前全局触发充值金额为 {{amount}}；超过全局默认充值金额 3 倍仍可保存，但可能存在较大风险。',
              {
                amount:
                  typeof selectedPool?.system_auto_recharge?.threshold ===
                  'number'
                    ? renderQuota(selectedPool.system_auto_recharge.threshold)
                    : '-',
              },
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
                label={t('月度自动充值')}
                onChange={setMonthlyRefillEnabled}
              />
              {monthlyRefillEnabled && (
                <Form.Switch
                  field='monthly_refill_top_up'
                  label={t('额度补齐')}
                />
              )}
              <Form.InputNumber
                field='monthly_refill_amount'
                label={t('月度充值金额')}
                extraText={t(
                  '关闭额度补齐时，每月固定增加该金额；开启后，该金额表示执行日的目标可用额度。',
                )}
              />
              <Form.InputNumber
                field='monthly_refill_day'
                label={t('执行日')}
                min={1}
                max={28}
                extraText={t(
                  '每月仅执行一次；即使无需补充额度，也会记录为本月已执行。',
                )}
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
            placeholder={t('搜索用户 ID、用户名、显示名称、邮箱或部门')}
            style={{ width: '100%' }}
            optionList={candidates.map((user) => ({
              label: formatCandidateLabel(user),
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
                '成员仅加入本额度池并消耗本池额度；池管理员可查看本池、添加默认池用户并给成员充值。',
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
