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
import {
  Button,
  Empty,
  ImagePreview,
  Input,
  Pagination,
  Select,
  Table,
  Tag,
} from '@douyinfe/semi-ui';
import { IconCopy, IconSearch } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { Layers3, Server, Wallet } from 'lucide-react';
import { useModelPricingData } from '../../hooks/model-pricing/useModelPricingData';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  calculateModelPrice,
  getLobeHubIcon,
  getModelPriceItems,
  renderModelTag,
} from '../../helpers';
import ModelDetailSideSheet from '../../components/table/model-pricing/modal/ModelDetailSideSheet';

function renderQuotaType(quotaType, t) {
  if (quotaType === 1) {
    return (
      <span className='console-pricing__billingTag console-pricing__billingTag--fixed'>
        {t('按次计费')}
      </span>
    );
  }

  return (
    <span className='console-pricing__billingTag console-pricing__billingTag--usage'>
      {t('按量计费')}
    </span>
  );
}

const ConsolePricing = () => {
  const isMobile = useIsMobile();
  const pricingData = useModelPricingData();

  const {
    t,
    filteredModels,
    loading,
    selectedRowKeys,
    setSelectedRowKeys,
    pageSize,
    setPageSize,
    currentPage,
    setCurrentPage,
    searchValue,
    handleChange,
    handleCompositionStart,
    handleCompositionEnd,
    copyText,
    displayPrice,
    groupRatio,
    selectedGroup,
    setSelectedGroup,
    setFilterGroup,
    filterQuotaType,
    setFilterQuotaType,
    filterEndpointType,
    setFilterEndpointType,
    models,
    showModelDetail,
    selectedModel,
    openModelDetail,
    closeModelDetail,
    modalImageUrl,
    isModalOpenurl,
    setIsModalOpenurl,
    currency,
    siteDisplayType,
    tokenUnit,
    usableGroup,
    vendorsMap,
    endpointMap,
    autoGroups,
  } = pricingData;

  const showRatio = false;

  const vendorCount = useMemo(() => {
    return new Set(
      models
        .map((model) => model.vendor_name)
        .filter((vendorName) => Boolean(vendorName)),
    ).size;
  }, [models]);

  const endpointCount = useMemo(() => {
    const endpointTypes = new Set();
    models.forEach((model) => {
      (model.supported_endpoint_types || []).forEach((endpoint) => {
        if (endpoint) {
          endpointTypes.add(endpoint);
        }
      });
    });
    return endpointTypes.size;
  }, [models]);

  const groupOptions = useMemo(() => {
    const groups = Object.keys(usableGroup || {}).filter(Boolean);
    return [
      { value: 'all', label: t('全部分组') },
      ...groups.map((group) => ({
        value: group,
        label: group,
      })),
    ];
  }, [usableGroup, t]);

  const quotaOptions = useMemo(
    () => [
      { value: 'all', label: t('全部计费') },
      { value: 0, label: t('按量计费') },
      { value: 1, label: t('按次计费') },
    ],
    [t],
  );

  const endpointOptions = useMemo(() => {
    const endpointTypes = new Set();
    models.forEach((model) => {
      (model.supported_endpoint_types || []).forEach((endpoint) => {
        if (endpoint) {
          endpointTypes.add(endpoint);
        }
      });
    });

    return [
      { value: 'all', label: t('全部端点') },
      ...Array.from(endpointTypes)
        .sort((a, b) => a.localeCompare(b))
        .map((endpoint) => ({
          value: endpoint,
          label: endpoint,
        })),
    ];
  }, [models, t]);

  const pageSizeOptions = useMemo(
    () =>
      [10, 20, 50, 100].map((value) => ({
        value,
        label: `${t('每页条数')}: ${value}`,
      })),
    [t],
  );

  const paginatedModels = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredModels.slice(start, start + pageSize);
  }, [filteredModels, currentPage, pageSize]);

  const pageStart =
    filteredModels.length === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const pageEnd = Math.min(currentPage * pageSize, filteredModels.length);

  const summaryCards = useMemo(
    () => [
      {
        key: 'models',
        icon: Layers3,
        label: t('当前模型数'),
        value: filteredModels.length,
        hint: `${models.length} ${t('总模型')}`,
      },
      {
        key: 'vendors',
        icon: Wallet,
        label: t('供应商数量'),
        value: vendorCount,
        hint: t('已接入的模型来源'),
      },
      {
        key: 'endpoints',
        icon: Server,
        label: t('端点类型'),
        value: endpointCount,
        hint: t('支持的调用能力'),
      },
    ],
    [endpointCount, filteredModels.length, models.length, t, vendorCount],
  );

  const handleCopySelectedModels = async () => {
    if (selectedRowKeys.length === 0) {
      return;
    }
    await copyText(selectedRowKeys.join('\n'));
  };

  const tableColumns = useMemo(
    () => [
      {
        title: t('模型名称'),
        dataIndex: 'model_name',
        width: isMobile ? 220 : 300,
        render: (value, record) => (
          <button
            type='button'
            className='console-pricing__modelButton'
            onClick={(event) => {
              event.stopPropagation();
              openModelDetail(record);
            }}
          >
            <span className='console-pricing__modelTagWrap'>
              {renderModelTag(value, {
                color: 'cyan',
                shape: 'circle',
                size: 'small',
                className: 'console-pricing__modelTag',
              })}
            </span>
            {record.vendor_name && (
              <span className='console-pricing__modelMeta'>
                {getLobeHubIcon(record.vendor_icon || 'Layers', 14)}
                <span>{record.vendor_name}</span>
              </span>
            )}
          </button>
        ),
      },
      {
        title: t('计费类型'),
        dataIndex: 'quota_type',
        width: isMobile ? 120 : 150,
        render: (value) => renderQuotaType(Number(value), t),
      },
      {
        title: t('可用端点类型'),
        dataIndex: 'supported_endpoint_types',
        width: isMobile ? 180 : 260,
        render: (value) => {
          if (!value || value.length === 0) {
            return '-';
          }

          return (
            <div className='console-pricing__endpointList'>
              {value.map((endpoint) => (
                <span key={endpoint} className='console-pricing__endpointTag'>
                  {endpoint}
                </span>
              ))}
            </div>
          );
        },
      },
      {
        title: t('模型价格'),
        dataIndex: 'model_price',
        render: (_, record) => {
          const priceData = calculateModelPrice({
            record,
            selectedGroup,
            groupRatio,
            tokenUnit,
            displayPrice,
            currency,
            quotaDisplayType: siteDisplayType,
            precision: 3,
          });
          const items = getModelPriceItems(priceData, t, siteDisplayType);

          return (
            <div className='console-pricing__priceBlock'>
              {items.map((item) => (
                <div key={item.key} className='console-pricing__priceLine'>
                  <span className='console-pricing__priceLabel'>
                    {item.label}
                  </span>
                  <span>
                    {item.value}
                    {item.suffix}
                  </span>
                </div>
              ))}
            </div>
          );
        },
      },
    ],
    [
      currency,
      displayPrice,
      groupRatio,
      isMobile,
      openModelDetail,
      selectedGroup,
      siteDisplayType,
      t,
      tokenUnit,
    ],
  );

  return (
    <div className='console-pricing-page mt-[60px]'>
      <div className='console-pricing__shell'>
        <section className='console-pricing__hero'>
          <div className='console-pricing__heroInner'>
            <div className='console-pricing__heroHeading'>
              <div>
                <div className='console-pricing__eyebrow'>{t('控制台')}</div>
                <h1 className='console-pricing__title'>{t('模型价格')}</h1>
                <p className='console-pricing__description'>
                  {t(
                    '在控制台中快速检索当前可用模型、计费方式、端点能力与价格信息。',
                  )}
                </p>
              </div>

              <div className='console-pricing__heroMeta'>
                <Tag
                  color='cyan'
                  shape='circle'
                  className='console-pricing__heroTag'
                >
                  {selectedGroup === 'all' ? t('智能最优分组') : selectedGroup}
                </Tag>
                <span className='console-pricing__heroTip'>
                  {t('点击任意模型行可查看完整价格详情')}
                </span>
              </div>
            </div>

            <div className='console-pricing__summaryGrid'>
              {summaryCards.map(({ key, icon: Icon, label, value, hint }) => (
                <article key={key} className='console-pricing__summaryCard'>
                  <div className='console-pricing__summaryIcon'>
                    <Icon size={18} />
                  </div>
                  <div className='console-pricing__summaryLabel'>{label}</div>
                  <div className='console-pricing__summaryValue'>{value}</div>
                  <div className='console-pricing__summaryHint'>{hint}</div>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className='console-pricing__panel'>
          <div className='console-pricing__toolbar'>
            <div className='console-pricing__toolbarItem console-pricing__toolbarItem--search'>
              <Input
                prefix={<IconSearch />}
                placeholder={t('搜索模型名称、标签或供应商')}
                value={searchValue}
                onCompositionStart={handleCompositionStart}
                onCompositionEnd={handleCompositionEnd}
                onChange={handleChange}
                showClear
              />
            </div>

            <div className='console-pricing__toolbarItem'>
              <Select
                value={selectedGroup}
                optionList={groupOptions}
                onChange={(value) => {
                  setSelectedGroup(value);
                  setFilterGroup(value);
                }}
              />
            </div>

            <div className='console-pricing__toolbarItem'>
              <Select
                value={filterQuotaType}
                optionList={quotaOptions}
                onChange={(value) => setFilterQuotaType(value)}
              />
            </div>

            <div className='console-pricing__toolbarItem'>
              <Select
                value={filterEndpointType}
                optionList={endpointOptions}
                onChange={(value) => setFilterEndpointType(value)}
              />
            </div>

            <Button
              theme='solid'
              type='primary'
              icon={<IconCopy />}
              className='console-pricing__copyButton'
              disabled={selectedRowKeys.length === 0}
              onClick={handleCopySelectedModels}
            >
              {t('复制选中')}
            </Button>
          </div>

          <div className='console-pricing__tableHeader'>
            <div className='console-pricing__tableMeta'>
              <span>{t('共 {{count}} 个模型', { count: filteredModels.length })}</span>
              <span className='console-pricing__tableMetaDivider'>/</span>
              <span>
                {t('当前显示')} {pageStart}-{pageEnd}
              </span>
            </div>

            <Select
              value={pageSize}
              optionList={pageSizeOptions}
              className='console-pricing__pageSizeSelect'
              onChange={(value) => {
                setPageSize(value);
                setCurrentPage(1);
              }}
            />
          </div>

          <div className='console-pricing__tableWrap'>
            <Table
              columns={tableColumns}
              dataSource={paginatedModels}
              loading={loading}
              pagination={false}
              rowSelection={{
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys),
              }}
              scroll={{ x: isMobile ? 760 : 'max-content' }}
              onRow={(record) => ({
                onClick: () => openModelDetail(record),
                style: { cursor: 'pointer' },
              })}
              empty={
                <Empty
                  image={
                    <IllustrationNoResult style={{ width: 150, height: 150 }} />
                  }
                  darkModeImage={
                    <IllustrationNoResultDark
                      style={{ width: 150, height: 150 }}
                    />
                  }
                  description={t('搜索无结果')}
                  style={{ padding: 36 }}
                />
              }
            />
          </div>

          <div className='console-pricing__pager'>
            <div className='console-pricing__pagerText'>
              {t('显示第')} {pageStart} - {pageEnd} {t('条，共')}{' '}
              {filteredModels.length} {t('条')}
            </div>

            <Pagination
              total={filteredModels.length}
              currentPage={currentPage}
              pageSize={pageSize}
              onPageChange={setCurrentPage}
              className='console-pricing__pagination'
            />
          </div>
        </section>
      </div>

      <ImagePreview
        src={modalImageUrl}
        visible={isModalOpenurl}
        onVisibleChange={(visible) => setIsModalOpenurl(visible)}
      />

      <ModelDetailSideSheet
        className='console-pricing__detailSheet'
        visible={showModelDetail}
        onClose={closeModelDetail}
        modelData={selectedModel}
        groupRatio={groupRatio}
        usableGroup={usableGroup}
        currency={currency}
        siteDisplayType={siteDisplayType}
        tokenUnit={tokenUnit}
        displayPrice={displayPrice}
        showRatio={showRatio}
        vendorsMap={vendorsMap}
        endpointMap={endpointMap}
        autoGroups={autoGroups}
        t={t}
      />
    </div>
  );
};

export default ConsolePricing;
