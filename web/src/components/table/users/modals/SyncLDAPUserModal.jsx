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

import React, { useEffect, useRef, useState } from 'react';
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
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconClose, IconRefresh, IconSearch, IconUserAdd } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

const SyncLDAPUserModal = (props) => {
  const { t } = useTranslation();
  const formApiRef = useRef(null);
  const [searching, setSearching] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [candidates, setCandidates] = useState([]);
  const [selectedKeys, setSelectedKeys] = useState([]);
  const [hasSearched, setHasSearched] = useState(false);
  const isMobile = useIsMobile();

  const getInitValues = () => ({
    username: '',
  });

  const resetState = () => {
    formApiRef.current?.setValues(getInitValues());
    setCandidates([]);
    setSelectedKeys([]);
    setHasSearched(false);
    setSearching(false);
    setSyncing(false);
  };

  useEffect(() => {
    if (!props.visible) {
      resetState();
    }
  }, [props.visible]);

  const search = async (values) => {
    setSearching(true);
    setCandidates([]);
    setSelectedKeys([]);
    setHasSearched(false);
    const res = await API.post('/api/user/ldap/sync', {
      action: 'search',
      username: values.username,
    });
    const { success, message } = res.data;
    if (success) {
      const users = res.data.data?.users || [];
      setCandidates(users);
      setHasSearched(true);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const syncSelected = async () => {
    if (selectedKeys.length === 0) {
      showError(t('请选择一个要同步的 LDAP 用户'));
      return;
    }
    setSyncing(true);
    const res = await API.post('/api/user/ldap/sync', {
      action: 'sync',
      email: selectedKeys[0],
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('LDAP 用户同步成功'));
      props.refresh();
      props.handleClose();
    } else {
      showError(message);
    }
    setSyncing(false);
  };

  const handleCancel = () => {
    props.handleClose();
  };

  const columns = [
    {
      title: t('用户名'),
      dataIndex: 'username',
      render: (text) => text || '-',
    },
    {
      title: t('邮箱'),
      dataIndex: 'email',
      render: (text) => text || '-',
    },
    {
      title: t('部门'),
      dataIndex: 'department',
      render: (text) => text || '-',
    },
  ];

  const rowSelection = {
    selectedRowKeys: selectedKeys,
    onChange: (selectedRowKeys) => {
      setSelectedKeys(selectedRowKeys.slice(-1));
    },
    getCheckboxProps: (record) => ({
      disabled: !record.email,
    }),
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
      width={isMobile ? '100%' : 760}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              onClick={syncSelected}
              icon={<IconRefresh />}
              loading={syncing}
              disabled={selectedKeys.length === 0}
            >
              {t('同步所选')}
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
      <Spin spinning={searching || syncing}>
        <Form
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={search}
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
                    {t('从 LDAP 查询并同步用户')}
                  </Text>
                  <div className='text-xs text-gray-600'>
                    {t('先查询 LDAP 用户，选择一个有邮箱的账号后同步')}
                  </div>
                </div>
              </div>

              <div className='flex flex-col md:flex-row md:items-end gap-2'>
                <div className='flex-1'>
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
                </div>
                <Button
                  theme='solid'
                  type='primary'
                  icon={<IconSearch />}
                  loading={searching}
                  onClick={() => formApiRef.current?.submitForm()}
                  className='mb-3'
                >
                  {t('查询')}
                </Button>
              </div>

              {hasSearched && (
                <div className='mt-3'>
                  <div className='mb-2 text-xs text-gray-600'>
                    {t('查询到 {{count}} 个 LDAP 用户，已选择 {{selected}} 个', {
                      count: candidates.length,
                      selected: selectedKeys.length,
                    })}
                  </div>
                  <Table
                    size='small'
                    rowKey='key'
                    columns={columns}
                    dataSource={candidates}
                    pagination={false}
                    rowSelection={rowSelection}
                    empty={t('未找到 LDAP 用户')}
                  />
                </div>
              )}
            </Card>
          </div>
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default SyncLDAPUserModal;
