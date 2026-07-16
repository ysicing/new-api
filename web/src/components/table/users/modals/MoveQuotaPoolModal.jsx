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

import React, { useMemo } from 'react';
import { Button, Form, Modal, Typography } from '@douyinfe/semi-ui';

const MoveQuotaPoolModal = ({
  visible,
  onCancel,
  onConfirm,
  user,
  quotaPools,
  t,
}) => {
  const targetPoolOptions = useMemo(() => {
    const currentPoolId = user?.quota_pool_id || 0;
    return quotaPools
      .map((pool) => ({
        ...pool,
        targetValue: pool.is_default ? 0 : pool.id,
      }))
      .filter(
        (pool) =>
          pool.targetValue !== currentPoolId &&
          (pool.is_default || pool.enabled),
      )
      .map((pool) => ({
        label: pool.name || t('默认额度池'),
        value: pool.targetValue,
      }));
  }, [quotaPools, t, user?.quota_pool_id]);

  return (
    <Modal
      title={t('调整额度池')}
      visible={visible}
      onCancel={onCancel}
      footer={null}
    >
      <Form
        onSubmit={async (values) => {
          const success = await onConfirm(user.id, values.pool_id);
          if (success) {
            onCancel();
          }
        }}
      >
        <Form.Slot label={t('用户')}>
          <Typography.Text>
            {user?.id} {user?.username}
          </Typography.Text>
        </Form.Slot>
        <Form.Select
          field='pool_id'
          label={t('目标额度池')}
          optionList={targetPoolOptions}
          rules={[{ required: true }]}
        />
        <Typography.Paragraph type='secondary'>
          {t('迁移会清零用户当前额度，并按规则退回原池余额。')}
        </Typography.Paragraph>
        <Button htmlType='submit' disabled={targetPoolOptions.length === 0}>
          {t('提交')}
        </Button>
      </Form>
    </Modal>
  );
};

export default MoveQuotaPoolModal;
