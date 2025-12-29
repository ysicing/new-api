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

import { useState, useCallback } from 'react';
import { API } from '../../helpers';
import { showError } from '../../helpers';

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
