import React, { useState, useEffect, useCallback } from 'react';
import {
  Table,
  Button,
  Modal,
  Form,
  Popconfirm,
  Space,
  Tag,
  Typography,
  Spin,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete, IconEdit } from '@douyinfe/semi-icons';
import {
  API,
  showError,
  showSuccess,
  getModelCategories,
  selectFilter,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const TokenBucketSettings = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [buckets, setBuckets] = useState([]);
  const [models, setModels] = useState([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingBucket, setEditingBucket] = useState(null);
  const [formApi, setFormApi] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const loadBuckets = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/token_bucket/');
      if (res.data.success) {
        setBuckets(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('获取令牌桶列表失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  const loadModels = useCallback(async () => {
    try {
      const res = await API.get('/api/user/models');
      const { success, data } = res.data;
      if (success) {
        const categories = getModelCategories(t);
        setModels(
          data.map((model) => {
            let icon = null;
            for (const [key, category] of Object.entries(categories)) {
              if (key !== 'all' && category.filter({ model_name: model })) {
                icon = category.icon;
                break;
              }
            }
            return {
              label: (
                <span className='flex items-center gap-1'>
                  {icon}
                  {model}
                </span>
              ),
              value: model,
            };
          }),
        );
      }
    } catch {
      // ignore
    }
  }, [t]);

  useEffect(() => {
    loadBuckets();
    loadModels();
  }, [loadBuckets, loadModels]);

  const handleCreate = () => {
    setEditingBucket(null);
    setModalVisible(true);
  };

  const handleEdit = (record) => {
    setEditingBucket(record);
    setModalVisible(true);
  };

  const handleDelete = async (id) => {
    try {
      const res = await API.delete(`/api/token_bucket/${id}`);
      if (res.data.success) {
        showSuccess(t('删除成功'));
        loadBuckets();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('删除失败'));
    }
  };

  const handleSubmit = async () => {
    if (!formApi) return;
    try {
      await formApi.validate();
    } catch {
      return;
    }
    const values = formApi.getValues();
    setSubmitting(true);
    try {
      const modelLimits = Array.isArray(values.model_limits)
        ? values.model_limits.join(',')
        : values.model_limits || '';
      const payload = {
        ...values,
        model_limits: modelLimits,
      };
      let res;
      if (editingBucket) {
        payload.id = editingBucket.id;
        res = await API.put('/api/token_bucket/', payload);
      } else {
        res = await API.post('/api/token_bucket/', payload);
      }
      if (res.data.success) {
        showSuccess(editingBucket ? t('更新成功') : t('创建成功'));
        setModalVisible(false);
        loadBuckets();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('操作失败'));
    } finally {
      setSubmitting(false);
    }
  };

  const getFormInitValues = () => {
    if (editingBucket) {
      return {
        name: editingBucket.name,
        ratio: editingBucket.ratio,
        model_limits: editingBucket.model_limits
          ? editingBucket.model_limits.split(',').filter(Boolean)
          : [],
      };
    }
    return { name: '', ratio: 1, model_limits: [] };
  };

  const columns = [
    {
      title: t('名称'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('倍率'),
      dataIndex: 'ratio',
      key: 'ratio',
      render: (val) => <Tag color='blue'>{val}</Tag>,
    },
    {
      title: t('模型限制'),
      dataIndex: 'model_limits',
      key: 'model_limits',
      render: (val) => {
        if (!val) return <Text type='tertiary'>{t('无限制')}</Text>;
        const mdls = val.split(',').filter(Boolean);
        if (mdls.length === 0)
          return <Text type='tertiary'>{t('无限制')}</Text>;
        return (
          <Space wrap>
            {mdls.slice(0, 3).map((m) => (
              <Tag key={m} size='small'>
                {m}
              </Tag>
            ))}
            {mdls.length > 3 && (
              <Tag size='small' color='grey'>
                +{mdls.length - 3}
              </Tag>
            )}
          </Space>
        );
      },
    },
    {
      title: t('操作'),
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            icon={<IconEdit />}
            size='small'
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title={t('确认删除')}
            content={t('删除后不可恢复')}
            onConfirm={() => handleDelete(record.id)}
          >
            <Button icon={<IconDelete />} size='small' type='danger' />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Spin spinning={loading}>
      <div style={{ marginBottom: 16 }}>
        <Button icon={<IconPlus />} theme='solid' onClick={handleCreate}>
          {t('新建令牌桶')}
        </Button>
      </div>
      <Table
        columns={columns}
        dataSource={buckets}
        rowKey='id'
        pagination={false}
        size='small'
      />
      <Modal
        title={editingBucket ? t('编辑令牌桶') : t('新建令牌桶')}
        visible={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        confirmLoading={submitting}
        closeOnEsc
      >
        <Form
          getFormApi={setFormApi}
          initValues={getFormInitValues()}
          key={editingBucket ? editingBucket.id : 'new'}
        >
          <Form.Input
            field='name'
            label={t('名称')}
            rules={[{ required: true, message: t('请输入名称') }]}
          />
          <Form.InputNumber
            field='ratio'
            label={t('倍率')}
            min={0.01}
            step={0.1}
            rules={[{ required: true, message: t('请输入倍率') }]}
          />
          <Form.Select
            field='model_limits'
            label={t('模型限制')}
            placeholder={t('请选择模型，留空表示不限制')}
            multiple
            optionList={models}
            filter={selectFilter}
            autoClearSearchValue={false}
            searchPosition='dropdown'
            showClear
            style={{ width: '100%' }}
          />
        </Form>
      </Modal>
    </Spin>
  );
};

export default TokenBucketSettings;
