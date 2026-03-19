import { useState, useCallback } from 'react';
import { API, showError } from '../../helpers';

export const useTopUsersData = () => {
  const [loading, setLoading] = useState(false);
  const [topUsers, setTopUsers] = useState([]);
  const [startTimestamp, setStartTimestamp] = useState(0);
  const [endTimestamp, setEndTimestamp] = useState(0);
  const [limit, setLimit] = useState(10);

  const fetchTopUsers = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (startTimestamp > 0) params.append('start_timestamp', startTimestamp);
      if (endTimestamp > 0) params.append('end_timestamp', endTimestamp);
      if (limit > 0) params.append('limit', limit);

      const res = await API.get(`/api/log/top_users?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        setTopUsers(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [startTimestamp, endTimestamp, limit]);

  return {
    loading,
    topUsers,
    startTimestamp,
    endTimestamp,
    limit,
    setStartTimestamp,
    setEndTimestamp,
    setLimit,
    fetchTopUsers,
  };
};
