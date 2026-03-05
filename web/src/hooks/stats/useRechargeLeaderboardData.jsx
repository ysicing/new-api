import { useState, useCallback } from 'react';
import { API, showError } from '../../helpers';

export const useRechargeLeaderboardData = () => {
  const [loading, setLoading] = useState(false);
  const [leaderboard, setLeaderboard] = useState([]);
  const [limit, setLimit] = useState(10);

  const fetchLeaderboard = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/log/recharge_leaderboard?limit=${limit}`);
      const { success, message, data } = res.data;
      if (success) {
        setLeaderboard(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [limit]);

  return {
    loading,
    leaderboard,
    limit,
    setLimit,
    fetchLeaderboard,
  };
};
