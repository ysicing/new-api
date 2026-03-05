import { useState, useCallback } from 'react';
import { API, showError, showSuccess } from '../../helpers';
import { useTranslation } from 'react-i18next';

export const useRechargeLeaderboardData = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [leaderboard, setLeaderboard] = useState([]);
  const [weeklyLimit, setWeeklyLimit] = useState(0);
  const [limit, setLimit] = useState(10);

  const fetchLeaderboard = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/log/recharge_leaderboard?limit=${limit}`);
      const { success, message, data } = res.data;
      if (success) {
        setLeaderboard(data?.list || []);
        setWeeklyLimit(data?.weekly_limit || 0);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [limit]);

  const rechargeUser = useCallback(async (userId) => {
    try {
      const res = await API.post('/api/user/manage', {
        id: userId,
        action: 'recharge_auto',
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        await fetchLeaderboard();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  }, [fetchLeaderboard, t]);

  return {
    loading,
    leaderboard,
    weeklyLimit,
    limit,
    setLimit,
    fetchLeaderboard,
    rechargeUser,
  };
};
