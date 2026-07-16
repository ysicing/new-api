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

import { useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  isAdmin,
  isQuotaPoolSuperAdminRole,
  isRoot,
  showError,
  showSuccess,
} from '../../helpers';
import { UserContext } from '../../context/User';

const getDefaultTransactionDateRange = () => {
  const end = new Date();
  const start = new Date(end.getTime() - 7 * 24 * 60 * 60 * 1000);
  return [start, end];
};

const toUnixTimestamp = (value) => {
  if (!value) return 0;
  const timestamp = value instanceof Date ? value.getTime() : Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : Math.floor(timestamp / 1000);
};

const buildTransactionParams = (page, pageSize, filters) => {
  const params = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  });
  const user = filters.user?.trim();
  if (user) {
    params.append('user', user);
  }
  if (filters.transactionType) {
    params.append('transaction_type', filters.transactionType);
  }
  const [startTime, endTime] = filters.dateRange || [];
  const startTimestamp = toUnixTimestamp(startTime);
  const endTimestamp = toUnixTimestamp(endTime);
  if (startTimestamp > 0) {
    params.append('start_timestamp', String(startTimestamp));
  }
  if (endTimestamp > 0) {
    params.append('end_timestamp', String(endTimestamp));
  }
  return params.toString();
};

const buildOperationLogParams = (page, pageSize, filters) => {
  const params = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  });
  const keyword = filters.keyword?.trim();
  if (keyword) {
    params.append('keyword', keyword);
  }
  const [startTime, endTime] = filters.dateRange || [];
  const startTimestamp = toUnixTimestamp(startTime);
  const endTimestamp = toUnixTimestamp(endTime);
  if (startTimestamp > 0) {
    params.append('start_timestamp', String(startTimestamp));
  }
  if (endTimestamp > 0) {
    params.append('end_timestamp', String(endTimestamp));
  }
  return params.toString();
};

export const useQuotaPoolsData = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const currentUser = useMemo(() => {
    if (userState?.user) {
      return userState.user;
    }
    try {
      return JSON.parse(localStorage.getItem('user') || '{}');
    } catch (error) {
      return {};
    }
  }, [userState?.user]);
  const systemAdmin = isAdmin();
  const root = isRoot();
  const quotaPoolSuperAdmin = isQuotaPoolSuperAdminRole();
  const poolAdmin = currentUser.quota_pool_admin;
  const canUseGlobalApi = systemAdmin || quotaPoolSuperAdmin;
  const canConfigurePools = root;
  const canConfigureRechargeRules =
    canConfigurePools ||
    systemAdmin ||
    quotaPoolSuperAdmin ||
    poolAdmin?.level >= 1;
  const canManagePoolAdmins = systemAdmin || quotaPoolSuperAdmin;
  const canManagePoolMembers =
    systemAdmin ||
    quotaPoolSuperAdmin ||
    (!canUseGlobalApi && poolAdmin?.level >= 1);
  const canRefillPools = systemAdmin;
  const canViewPoolManagement = canUseGlobalApi || poolAdmin?.level >= 1;

  const [loading, setLoading] = useState(false);
  const [pools, setPools] = useState([]);
  const [defaultPool, setDefaultPool] = useState(null);
  const [selectedPool, setSelectedPool] = useState(null);
  const [adminContacts, setAdminContacts] = useState([]);
  const [members, setMembers] = useState([]);
  const [membersPage, setMembersPage] = useState(1);
  const [membersPageSize, setMembersPageSize] = useState(20);
  const [membersTotal, setMembersTotal] = useState(0);
  const [transactions, setTransactions] = useState([]);
  const [transactionsPage, setTransactionsPage] = useState(1);
  const [transactionsPageSize, setTransactionsPageSize] = useState(20);
  const [transactionsTotal, setTransactionsTotal] = useState(0);
  const [operationLogs, setOperationLogs] = useState([]);
  const [operationLogsPage, setOperationLogsPage] = useState(1);
  const [operationLogsPageSize, setOperationLogsPageSize] = useState(20);
  const [operationLogsTotal, setOperationLogsTotal] = useState(0);
  const [transactionFilters, setTransactionFilters] = useState(() => ({
    user: '',
    transactionType: '',
    dateRange: getDefaultTransactionDateRange(),
  }));
  const [appliedTransactionFilters, setAppliedTransactionFilters] = useState(
    () => ({
      user: '',
      transactionType: '',
      dateRange: getDefaultTransactionDateRange(),
    }),
  );
  const [operationLogFilters, setOperationLogFilters] = useState(() => ({
    keyword: '',
    dateRange: getDefaultTransactionDateRange(),
  }));
  const [appliedOperationLogFilters, setAppliedOperationLogFilters] = useState(
    () => ({
      keyword: '',
      dateRange: getDefaultTransactionDateRange(),
    }),
  );
  const [candidates, setCandidates] = useState([]);
  const [stats, setStats] = useState({
    usage: [],
    total_usage: 0,
  });
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsPeriod, setStatsPeriod] = useState('week');

  const selectedPoolId = selectedPool?.id || poolAdmin?.pool_id || 0;

  const poolBaseUrl = useMemo(() => {
    if (canUseGlobalApi) return '/api/quota_pool';
    return '/api/quota_pool/self';
  }, [canUseGlobalApi]);

  const loadPools = useCallback(async () => {
    setLoading(true);
    try {
      if (canUseGlobalApi) {
        const res = await API.get('/api/quota_pool');
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          return;
        }
        const nextPools = data || [];
        setPools(nextPools);
        setDefaultPool(null);
        setAdminContacts([]);
        setSelectedPool((current) => {
          if (!current) {
            return null;
          }
          return nextPools.find((pool) => pool.id === current.id) || null;
        });
      } else {
        const res = await API.get('/api/quota_pool/self');
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          return;
        }
        setPools(data?.pool ? [data.pool] : []);
        setDefaultPool(data?.default_pool || null);
        setAdminContacts(data?.admin_contacts || []);
        setSelectedPool((current) => {
          if (!current) return data?.pool || null;
          return data?.pool || null;
        });
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  }, [canUseGlobalApi, t]);

  const loadMembers = useCallback(
    async (page, pageSize) => {
      if (!canViewPoolManagement || !selectedPoolId) {
        setMembers([]);
        setMembersTotal(0);
        return;
      }
      const url = canUseGlobalApi
        ? `/api/quota_pool/${selectedPoolId}/members`
        : '/api/quota_pool/self/members';
      const res = await API.get(`${url}?p=${page}&page_size=${pageSize}`);
      const { success, message, data } = res.data;
      if (success) {
        setMembers(data?.items || []);
        setMembersTotal(data?.total || 0);
      } else {
        showError(message);
      }
    },
    [canUseGlobalApi, canViewPoolManagement, selectedPoolId],
  );

  const loadTransactions = useCallback(
    async (page, pageSize, filters = appliedTransactionFilters) => {
      if (!canViewPoolManagement || !selectedPoolId) {
        setTransactions([]);
        setTransactionsTotal(0);
        return;
      }
      const url = canUseGlobalApi
        ? `/api/quota_pool/${selectedPoolId}/transactions`
        : '/api/quota_pool/self/transactions';
      const res = await API.get(
        `${url}?${buildTransactionParams(page, pageSize, filters)}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setTransactions(data?.items || []);
        setTransactionsTotal(data?.total || 0);
      } else {
        showError(message);
      }
    },
    [
      appliedTransactionFilters,
      canUseGlobalApi,
      canViewPoolManagement,
      selectedPoolId,
    ],
  );

  const loadOperationLogs = useCallback(
    async (page, pageSize, filters = appliedOperationLogFilters) => {
      if (!canViewPoolManagement || !selectedPoolId) {
        setOperationLogs([]);
        setOperationLogsTotal(0);
        return;
      }
      const url = canUseGlobalApi
        ? `/api/quota_pool/${selectedPoolId}/operation_logs`
        : '/api/quota_pool/self/operation_logs';
      const res = await API.get(
        `${url}?${buildOperationLogParams(page, pageSize, filters)}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setOperationLogs(data?.items || []);
        setOperationLogsTotal(data?.total || 0);
      } else {
        showError(message);
      }
    },
    [
      appliedOperationLogFilters,
      canUseGlobalApi,
      canViewPoolManagement,
      selectedPoolId,
    ],
  );

  const loadStats = useCallback(async () => {
    if (!canViewPoolManagement || !selectedPoolId) {
      setStats({
        usage: [],
        total_usage: 0,
      });
      return;
    }
    setStatsLoading(true);
    try {
      const url = canUseGlobalApi
        ? `/api/quota_pool/${selectedPoolId}/stats`
        : '/api/quota_pool/self/stats';
      const res = await API.get(`${url}?period=${statsPeriod}`);
      const { success, message, data } = res.data;
      if (success) {
        setStats({
          usage: data?.usage || [],
          total_usage: data?.total_usage || 0,
        });
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setStatsLoading(false);
    }
  }, [canUseGlobalApi, canViewPoolManagement, selectedPoolId, statsPeriod, t]);

  const loadCandidates = useCallback(
    async (keyword = '') => {
      if (!canManagePoolMembers && !canManagePoolAdmins) {
        setCandidates([]);
        return;
      }
      const url = canUseGlobalApi
        ? `/api/quota_pool/candidates?keyword=${encodeURIComponent(keyword)}`
        : `/api/quota_pool/self/candidates?keyword=${encodeURIComponent(keyword)}`;
      const res = await API.get(`${url}&p=1&page_size=20`);
      const { success, message, data } = res.data;
      if (success) {
        setCandidates(data?.items || []);
      } else {
        showError(message);
      }
    },
    [canManagePoolAdmins, canManagePoolMembers, canUseGlobalApi],
  );

  const refreshMembers = useCallback(async () => {
    await loadMembers(membersPage, membersPageSize);
  }, [loadMembers, membersPage, membersPageSize]);

  const refreshTransactions = useCallback(async () => {
    await loadTransactions(transactionsPage, transactionsPageSize);
  }, [loadTransactions, transactionsPage, transactionsPageSize]);

  const refreshOperationLogs = useCallback(async () => {
    await loadOperationLogs(operationLogsPage, operationLogsPageSize);
  }, [loadOperationLogs, operationLogsPage, operationLogsPageSize]);

  const refreshDetail = useCallback(async () => {
    await Promise.all([
      refreshMembers(),
      refreshTransactions(),
      refreshOperationLogs(),
      loadStats(),
    ]);
  }, [refreshMembers, refreshTransactions, refreshOperationLogs, loadStats]);

  const refreshLatestTransactions = useCallback(async () => {
    setTransactionsPage(1);
    await loadTransactions(1, transactionsPageSize);
  }, [loadTransactions, transactionsPageSize]);

  const handleMembersPageChange = useCallback((page) => {
    setMembersPage(page);
  }, []);

  const handleMembersPageSizeChange = useCallback((pageSize) => {
    setMembersPage(1);
    setMembersPageSize(pageSize);
  }, []);

  const handleTransactionsPageChange = useCallback((page) => {
    setTransactionsPage(page);
  }, []);

  const handleTransactionsPageSizeChange = useCallback((pageSize) => {
    setTransactionsPage(1);
    setTransactionsPageSize(pageSize);
  }, []);

  const handleOperationLogsPageChange = useCallback((page) => {
    setOperationLogsPage(page);
  }, []);

  const handleOperationLogsPageSizeChange = useCallback((pageSize) => {
    setOperationLogsPage(1);
    setOperationLogsPageSize(pageSize);
  }, []);

  const updateTransactionFilter = useCallback((field, value) => {
    setTransactionFilters((filters) => ({
      ...filters,
      [field]: value,
    }));
  }, []);

  const updateOperationLogFilter = useCallback((field, value) => {
    setOperationLogFilters((filters) => ({
      ...filters,
      [field]: value,
    }));
  }, []);

  const searchTransactions = useCallback(() => {
    setTransactionsPage(1);
    setAppliedTransactionFilters(transactionFilters);
  }, [transactionFilters]);

  const searchOperationLogs = useCallback(() => {
    setOperationLogsPage(1);
    setAppliedOperationLogFilters(operationLogFilters);
  }, [operationLogFilters]);

  const resetTransactionFilters = useCallback(() => {
    const filters = {
      user: '',
      transactionType: '',
      dateRange: getDefaultTransactionDateRange(),
    };
    setTransactionsPage(1);
    setTransactionFilters(filters);
    setAppliedTransactionFilters(filters);
  }, []);

  const resetOperationLogFilters = useCallback(() => {
    const filters = {
      keyword: '',
      dateRange: getDefaultTransactionDateRange(),
    };
    setOperationLogsPage(1);
    setOperationLogFilters(filters);
    setAppliedOperationLogFilters(filters);
  }, []);

  useEffect(() => {
    loadPools();
  }, [loadPools]);

  useEffect(() => {
    setMembersPage(1);
    setTransactionsPage(1);
    setOperationLogsPage(1);
  }, [selectedPoolId]);

  useEffect(() => {
    refreshMembers();
  }, [refreshMembers]);

  useEffect(() => {
    refreshTransactions();
  }, [refreshTransactions]);

  useEffect(() => {
    refreshOperationLogs();
  }, [refreshOperationLogs]);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  const createPool = async (values) => {
    const res = await API.post('/api/quota_pool', values);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
      await refreshOperationLogs();
    } else {
      showError(message);
    }
  };

  const syncDefaultPool = async () => {
    const res = await API.post('/api/quota_pool/sync_default');
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
    } else {
      showError(message);
    }
  };

  const updatePool = async (poolId, values) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${poolId}`
      : '/api/quota_pool/self';
    const res = await API.put(url, values);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
    } else {
      showError(message);
    }
  };

  const setPoolEnabled = async (poolId, enabled) => {
    const action = enabled ? 'enable' : 'disable';
    const res = await API.post(`/api/quota_pool/${poolId}/${action}`);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
      await refreshDetail();
    } else {
      showError(message);
    }
  };

  const deletePool = async (poolId) => {
    const res = await API.delete(`/api/quota_pool/${poolId}`);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
      await refreshDetail();
    } else {
      showError(message);
    }
  };

  const refillPool = async (poolId, amount) => {
    const res = await API.post(`/api/quota_pool/${poolId}/refill`, { amount });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
      await refreshDetail();
    } else {
      showError(message);
    }
  };

  const addMember = async (userId) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/members`
      : '/api/quota_pool/self/members';
    const res = await API.post(url, { user_id: userId });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message || t('操作成功完成！'));
      await loadPools();
      setMembersPage(1);
      await loadMembers(1, membersPageSize);
      await Promise.all([
        refreshLatestTransactions(),
        refreshOperationLogs(),
        loadStats(),
      ]);
    } else {
      showError(message);
    }
  };

  const moveMember = async (userId, poolId) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/users/${userId}`
      : `/api/quota_pool/self/members/${userId}`;
    const res = await API.put(url, {
      pool_id: poolId,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message || t('操作成功完成！'));
      await loadPools();
      await refreshDetail();
    } else {
      showError(message);
    }
  };

  const rechargeMember = async (userId) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/members/${userId}/recharge`
      : `/api/quota_pool/self/members/${userId}/recharge`;
    const res = await API.post(url);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
      await refreshDetail();
    } else {
      showError(message);
    }
  };

  const reclaimMember = async (userId) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/members/${userId}/reclaim`
      : `/api/quota_pool/self/members/${userId}/reclaim`;
    const res = await API.post(url);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
      await refreshDetail();
    } else {
      showError(message);
    }
  };

  const grantAdmin = async (userId, level) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/admins`
      : '/api/quota_pool/self/admins';
    const res = await API.post(url, { user_id: userId, level });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadMembers(membersPage, membersPageSize);
      await refreshOperationLogs();
    } else {
      showError(message);
    }
  };

  const revokeAdmin = async (userId) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/admins/${userId}`
      : `/api/quota_pool/self/admins/${userId}`;
    const res = await API.delete(url);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadMembers(membersPage, membersPageSize);
      await refreshOperationLogs();
    } else {
      showError(message);
    }
  };

  return {
    t,
    loading,
    pools,
    defaultPool,
    selectedPool,
    setSelectedPool,
    adminContacts,
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
    quotaPoolAdmin: poolAdmin,
    poolBaseUrl,
    refreshDetail,
  };
};
