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

import React, { useRef, useState } from 'react';
import { API, showError, showSuccess } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Avatar,
  Button,
  Card,
  Form,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconClose, IconRefresh, IconSearch, IconUserAdd } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

const SyncLDAPUserModal = (props) => {
  const { t } = useTranslation();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const isMobile = useIsMobile();

  const getInitValues = () => ({
    username: '',
  });

  const submit = async (values) => {
    setLoading(true);
    const res = await API.post('/api/user/ldap/sync', {
      username: values.username,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('LDAP 用户同步成功'));
      formApiRef.current?.setValues(getInitValues());
      props.refresh();
      props.handleClose();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const handleCancel = () => {
    props.handleClose();
  };

  return (
    <SideSheet
      placement='left'
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('同步')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('同步 LDAP 用户')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={props.visible}
      width={isMobile ? '100%' : 520}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconRefresh />}
              loading={loading}
            >
              {t('同步')}
            </Button>
            <Button
              theme='light'
              type='primary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={handleCancel}
    >
      <Spin spinning={loading}>
        <Form
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
          onSubmitFail={(errs) => {
            const first = Object.values(errs)[0];
            if (first) showError(Array.isArray(first) ? first[0] : first);
            formApiRef.current?.scrollToError();
          }}
        >
          <div className='p-2'>
            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center mb-2'>
                <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                  <IconUserAdd size={16} />
                </Avatar>
                <div>
                  <Text className='text-lg font-medium'>
                    {t('从 LDAP 同步指定用户')}
                  </Text>
                  <div className='text-xs text-gray-600'>
                    {t('按邮箱匹配已有账号，不存在则创建本地账号')}
                  </div>
                </div>
              </div>

              <Form.Input
                field='username'
                label={t('LDAP 用户名或邮箱')}
                placeholder={t('请输入 LDAP 用户名或邮箱')}
                prefix={<IconSearch />}
                rules={[
                  {
                    required: true,
                    message: t('请输入 LDAP 用户名或邮箱'),
                  },
                ]}
                showClear
              />
            </Card>
          </div>
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default SyncLDAPUserModal;
