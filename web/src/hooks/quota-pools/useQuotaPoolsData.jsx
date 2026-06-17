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
  const [transactions, setTransactions] = useState([]);
  const [candidates, setCandidates] = useState([]);

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
        setPools(data || []);
        if (!selectedPool && data?.length) {
          const firstNonDefault = data.find((pool) => !pool.is_default);
          setSelectedPool(firstNonDefault || data[0]);
        }
      } else {
        const res = await API.get('/api/quota_pool/self');
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          return;
        }
        setPools(data?.pool ? [data.pool] : []);
        setSelectedPool(data?.pool || null);
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  }, [canUseGlobalApi, selectedPool, t]);

  const loadMembers = useCallback(async () => {
    if (!selectedPoolId) return;
    const url = canUseGlobalApi
      ? `/api/quota_pool/${selectedPoolId}/members`
      : '/api/quota_pool/self/members';
    const res = await API.get(`${url}?p=1&page_size=100`);
    const { success, message, data } = res.data;
    if (success) {
      setMembers(data?.items || []);
    } else {
      showError(message);
    }
  }, [canUseGlobalApi, selectedPoolId]);

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

  const loadCandidates = useCallback(async (keyword = '') => {
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
  }, [canUseGlobalApi]);

  const refreshDetail = useCallback(async () => {
    await Promise.all([loadMembers(), loadTransactions()]);
  }, [loadMembers, loadTransactions]);

  useEffect(() => {
    loadPools();
  }, [loadPools]);

  useEffect(() => {
    refreshDetail();
  }, [refreshDetail]);

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
      await refreshDetail();
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
    transactions,
    candidates,
    loadCandidates,
    createPool,
    updatePool,
    refillPool,
    addMember,
    rechargeMember,
    grantAdmin,
    canUseGlobalApi,
    canConfigurePools,
    canManagePoolAdmins,
    canRefillPools,
    poolBaseUrl,
    refreshDetail,
  };
};
