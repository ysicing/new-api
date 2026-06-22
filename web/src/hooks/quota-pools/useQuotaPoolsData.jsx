import { useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, isAdmin, isRoot, showError, showSuccess } from '../../helpers';
import { UserContext } from '../../context/User';

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
  const poolAdmin = currentUser.quota_pool_admin;
  const canUseGlobalApi = systemAdmin;
  const canConfigurePools = root;
  const canManagePoolAdmins = systemAdmin || poolAdmin?.level >= 2;
  const canRefillPools = systemAdmin;

  const [loading, setLoading] = useState(false);
  const [pools, setPools] = useState([]);
  const [selectedPool, setSelectedPool] = useState(null);
  const [members, setMembers] = useState([]);
  const [membersPage, setMembersPage] = useState(1);
  const [membersPageSize, setMembersPageSize] = useState(20);
  const [membersTotal, setMembersTotal] = useState(0);
  const [transactions, setTransactions] = useState([]);
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
        setSelectedPool((current) => {
          if (!current) {
            return null;
          }
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
      if (!selectedPoolId) {
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
    [canUseGlobalApi, selectedPoolId],
  );

  const loadTransactions = useCallback(async () => {
    if (!selectedPoolId) return;
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/transactions`
      : '/api/quota_pool/self/transactions';
    const res = await API.get(`${url}?p=1&page_size=50`);
    const { success, message, data } = res.data;
    if (success) {
      setTransactions(data?.items || []);
    } else {
      showError(message);
    }
  }, [canUseGlobalApi, selectedPoolId]);

  const loadStats = useCallback(async () => {
    if (!selectedPoolId) {
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
  }, [canUseGlobalApi, selectedPoolId, statsPeriod, t]);

  const loadCandidates = useCallback(
    async (keyword = '') => {
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
    [canUseGlobalApi],
  );

  const refreshMembers = useCallback(async () => {
    await loadMembers(membersPage, membersPageSize);
  }, [loadMembers, membersPage, membersPageSize]);

  const refreshDetail = useCallback(async () => {
    await Promise.all([refreshMembers(), loadTransactions(), loadStats()]);
  }, [refreshMembers, loadTransactions, loadStats]);

  const handleMembersPageChange = useCallback((page) => {
    setMembersPage(page);
  }, []);

  const handleMembersPageSizeChange = useCallback((pageSize) => {
    setMembersPage(1);
    setMembersPageSize(pageSize);
  }, []);

  useEffect(() => {
    loadPools();
  }, [loadPools]);

  useEffect(() => {
    setMembersPage(1);
  }, [selectedPoolId]);

  useEffect(() => {
    refreshMembers();
  }, [refreshMembers]);

  useEffect(() => {
    loadTransactions();
  }, [loadTransactions]);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  const createPool = async (values) => {
    const res = await API.post('/api/quota_pool', values);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadPools();
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
    const res = await API.put(`/api/quota_pool/${poolId}`, values);
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
      await Promise.all([loadTransactions(), loadStats()]);
    } else {
      showError(message);
    }
  };

  const moveMember = async (userId, poolId) => {
    const res = await API.put(`/api/quota_pool/users/${userId}`, {
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

  const grantAdmin = async (userId, level) => {
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/admins`
      : '/api/quota_pool/self/admins';
    const res = await API.post(url, { user_id: userId, level });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      await loadMembers(membersPage, membersPageSize);
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
    } else {
      showError(message);
    }
  };

  return {
    t,
    loading,
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
    grantAdmin,
    revokeAdmin,
    canUseGlobalApi,
    canConfigurePools,
    canManagePoolAdmins,
    canRefillPools,
    poolBaseUrl,
    refreshDetail,
  };
};
