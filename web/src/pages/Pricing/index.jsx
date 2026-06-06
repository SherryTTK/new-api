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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import {
  Empty,
  ImagePreview,
  Input,
  Pagination,
  Select,
  Table,
} from '@douyinfe/semi-ui';
import {
  IconChevronLeft,
  IconChevronRight,
  IconCopy,
  IconSearch,
  IconServer,
} from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import MarketingShell from '../../components/marketing/MarketingShell';
import { useModelPricingData } from '../../hooks/model-pricing/useModelPricingData';
import { calculateModelPrice, getModelPriceItems } from '../../helpers';
import { getLobeHubIcon, renderModelTag } from '../../helpers';
import ModelDetailSideSheet from '../../components/table/model-pricing/modal/ModelDetailSideSheet';

const VENDOR_TAB_STEP = 360;

const Pricing = () => {
  const pricingData = useModelPricingData();
  const { t } = useTranslation();
  const vendorTabsRef = useRef(null);
  const [activeVendorTab, setActiveVendorTab] = useState('all');

  const {
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
    filterVendor,
    setFilterVendor,
    setFilterGroup,
    filterQuotaType,
    setFilterQuotaType,
    models,
    showModelDetail,
    selectedModel,
    openModelDetail,
    closeModelDetail,
    modalImageUrl,
    isModalOpenurl,
    setIsModalOpenurl,
    setModalImageUrl,
    currency,
    siteDisplayType,
    tokenUnit,
    showRatio,
    usableGroup,
    vendorsMap,
    endpointMap,
    autoGroups,
  } = pricingData;

  useEffect(() => {
    setActiveVendorTab(filterVendor);
  }, [filterVendor]);

  const vendorStats = useMemo(() => {
    const counters = new Map();
    let unknownCount = 0;

    models.forEach((model) => {
      if (model.vendor_name) {
        counters.set(model.vendor_name, (counters.get(model.vendor_name) || 0) + 1);
        return;
      }
      unknownCount += 1;
    });

    const vendorItems = [
      {
        key: 'all',
        label: t('全部模型'),
        count: models.length,
      },
      ...Array.from(counters.entries())
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([name, count]) => ({
          key: name,
          label: name,
          count,
          icon: models.find((model) => model.vendor_name === name)?.vendor_icon,
        })),
    ];

    if (unknownCount > 0) {
      vendorItems.push({
        key: 'unknown',
        label: t('未知供应商'),
        count: unknownCount,
      });
    }

    return vendorItems;
  }, [models, t]);

  const endpointCount = useMemo(() => {
    const endpointTypes = new Set();
    models.forEach((model) => {
      (model.supported_endpoint_types || []).forEach((endpoint) =>
        endpointTypes.add(endpoint),
      );
    });
    return endpointTypes.size;
  }, [models]);

  const paginatedModels = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredModels.slice(start, start + pageSize);
  }, [filteredModels, currentPage, pageSize]);

  const handleVendorTabClick = (vendorKey) => {
    setActiveVendorTab(vendorKey);
    setFilterVendor(vendorKey);
    setCurrentPage(1);
  };

  const handleCopySelectedModels = async () => {
    if (selectedRowKeys.length === 0) {
      return;
    }
    await copyText(selectedRowKeys.join('\n'));
  };

  const renderQuotaType = (quotaType) => {
    if (quotaType === 1) {
      return (
        <span className='landing-pricing__tag landing-pricing__tag--fixed'>
          {t('按次计费')}
        </span>
      );
    }
    return (
      <span className='landing-pricing__tag landing-pricing__tag--quota'>
        {t('按量计费')}
      </span>
    );
  };

  const getTableColumns = () => [
    {
      title: '',
      dataIndex: 'supported_endpoint_types',
      width: 168,
      render: (value) => {
        const endpoint = value?.[0] || 'api';
        return (
          <span className='landing-pricing__tag landing-pricing__tag--endpoint'>
            {endpoint}
          </span>
        );
      },
    },
    {
      title: t('模型名称'),
      dataIndex: 'model_name',
      width: 260,
      render: (value, record) => (
        <button
          type='button'
          className='landing-pricing__modelName'
          onClick={(event) => {
            event.stopPropagation();
            openModelDetail(record);
          }}
        >
          {renderModelTag(value, {
            color: 'violet',
            shape: 'circle',
            size: 'small',
          })}
        </button>
      ),
    },
    {
      title: t('计费类型'),
      dataIndex: 'quota_type',
      width: 170,
      render: (value) => renderQuotaType(Number(value)),
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
          <div className='landing-pricing__priceBlock'>
            {items.map((item) => (
              <div key={item.key} className='landing-pricing__priceLine'>
                <span className='landing-pricing__priceLabel'>{item.label}</span>{' '}
                {item.value}
                {item.suffix}
              </div>
            ))}
          </div>
        );
      },
    },
  ];

  const tableColumns = useMemo(
    () => getTableColumns(),
    [
      t,
      selectedGroup,
      groupRatio,
      tokenUnit,
      displayPrice,
      currency,
      siteDisplayType,
    ],
  );

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

  const quotaOptions = [
    { value: 'all', label: t('全部计费') },
    { value: 0, label: t('按量计费') },
    { value: 1, label: t('按次计费') },
  ];

  const pageSizeOptions = [10, 20, 50, 100].map((value) => ({
    value,
    label: `${t('每页条数')}: ${value}`,
  }));

  const pageStart = filteredModels.length === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const pageEnd = Math.min(currentPage * pageSize, filteredModels.length);

  return (
    <MarketingShell activeNav='pricing' className='landing-pricing'>
      <section className='landing-pricing__hero'>
        <div className='landing-pricing__heroText'>
          <h1 className='landing-pricing__heroTitle'>{t('模型定价')}</h1>
          <p className='landing-pricing__heroDescription'>
            {t('清晰查看不同模型的计费方式、可用端点和当前价格')}
          </p>
        </div>

        <div className='landing-pricing__heroCard'>
          <div className='landing-pricing__heroCardText'>
            <span className='landing-pricing__heroCardTitle'>
              {t('产品包更划算 >')}
            </span>
            <span className='landing-pricing__heroCardSubtitle'>
              {t('按业务阶段灵活选择合适的接入方式')}
            </span>
          </div>
          <Link to='/purchase' className='landing-pricing__heroButton'>
            {t('前往购买')}
          </Link>
        </div>
      </section>

      <section className='landing-pricing__content'>
        <div className='landing-pricing__panel'>
          <div className='landing-pricing__panelInner'>
            <div className='landing-pricing__summary'>
              <div className='landing-pricing__summaryMain'>
                <span className='landing-pricing__summaryIcon'>
                  <IconServer />
                </span>
                <div>
                  <h2 className='landing-pricing__summaryTitle'>{t('模型定价')}</h2>
                  <div className='landing-pricing__summaryMeta'>
                    <span>{t('可用分组')}:</span>
                    <span className='landing-pricing__summaryMainTag'>
                      {selectedGroup === 'all' ? 'default' : selectedGroup}
                    </span>
                  </div>
                </div>
              </div>

              <div className='landing-pricing__summaryCards'>
                <div className='landing-pricing__summaryCard'>
                  <div className='landing-pricing__summaryCardLabel'>
                    {t('可用模型')}
                  </div>
                  <div className='landing-pricing__summaryCardValue'>
                    {filteredModels.length}
                  </div>
                </div>
                <div className='landing-pricing__summaryCard'>
                  <div className='landing-pricing__summaryCardLabel'>
                    {t('计费类型')}
                  </div>
                  <div className='landing-pricing__summaryCardValue'>
                    {new Set(filteredModels.map((model) => model.quota_type)).size}
                  </div>
                </div>
                <div className='landing-pricing__summaryCard'>
                  <div className='landing-pricing__summaryCardLabel'>
                    {t('可用端点')}
                  </div>
                  <div className='landing-pricing__summaryCardValue'>
                    {endpointCount}
                  </div>
                </div>
              </div>
            </div>

            <div className='landing-pricing__vendorTabs'>
              <button
                type='button'
                className='landing-pricing__arrowButton'
                onClick={() => {
                  vendorTabsRef.current?.scrollBy({
                    left: -VENDOR_TAB_STEP,
                    behavior: 'smooth',
                  });
                }}
              >
                <IconChevronLeft />
              </button>

              <div className='landing-pricing__tabScroller' ref={vendorTabsRef}>
                {vendorStats.map((vendor) => (
                  <button
                    key={vendor.key}
                    type='button'
                    className={`landing-pricing__vendorTab${activeVendorTab === vendor.key ? ' landing-pricing__vendorTab--active' : ''}`}
                    onClick={() => handleVendorTabClick(vendor.key)}
                  >
                    {vendor.icon && getLobeHubIcon(vendor.icon, 14)}
                    <span>{vendor.label}</span>
                    <span className='landing-pricing__vendorTabCount'>
                      {vendor.count}
                    </span>
                  </button>
                ))}
              </div>

              <button
                type='button'
                className='landing-pricing__arrowButton'
                onClick={() => {
                  vendorTabsRef.current?.scrollBy({
                    left: VENDOR_TAB_STEP,
                    behavior: 'smooth',
                  });
                }}
              >
                <IconChevronRight />
              </button>
            </div>

            <div className='landing-pricing__toolbar'>
              <div className='landing-pricing__searchWrap'>
                <Input
                  prefix={<IconSearch />}
                  placeholder={t('模糊搜索模型名称')}
                  value={searchValue}
                  onCompositionStart={handleCompositionStart}
                  onCompositionEnd={handleCompositionEnd}
                  onChange={handleChange}
                  showClear
                />
              </div>

              <Select
                value={selectedGroup}
                optionList={groupOptions}
                onChange={(value) => {
                  setSelectedGroup(value);
                  setFilterGroup(value);
                }}
              />

              <Select
                value={filterQuotaType}
                optionList={quotaOptions}
                onChange={(value) => setFilterQuotaType(value)}
              />

              <button
                type='button'
                className='landing-home__primaryButton landing-pricing__copyButton'
                onClick={handleCopySelectedModels}
                disabled={selectedRowKeys.length === 0}
              >
                <IconCopy />
                {t('复制选中模型')}
              </button>
            </div>

            <div className='landing-pricing__tableWrap'>
              <Table
                columns={tableColumns}
                dataSource={paginatedModels}
                loading={loading}
                pagination={false}
                rowSelection={{
                  selectedRowKeys,
                  onChange: (keys) => setSelectedRowKeys(keys),
                }}
                scroll={{ x: 'max-content' }}
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
                    style={{ padding: 30 }}
                  />
                }
              />
            </div>

            <div className='landing-pricing__pager'>
              <div className='landing-pricing__pagerText'>
                {t('显示第')} {pageStart} - {pageEnd} {t('条，共')} {filteredModels.length}{' '}
                {t('条')}
              </div>

              <div className='landing-pricing__pagerControl'>
                <Pagination
                  total={filteredModels.length}
                  currentPage={currentPage}
                  pageSize={pageSize}
                  onPageChange={setCurrentPage}
                  className='landing-pricing__pagination'
                />

                <Select
                  value={pageSize}
                  optionList={pageSizeOptions}
                  className='landing-pricing__pageSizeSelect'
                  onChange={(value) => {
                    setPageSize(value);
                    setCurrentPage(1);
                  }}
                />
              </div>
            </div>
          </div>
        </div>
      </section>

      <ImagePreview
        src={modalImageUrl}
        visible={isModalOpenurl}
        onVisibleChange={(visible) => setIsModalOpenurl(visible)}
      />

      <ModelDetailSideSheet
        className='landing-pricing__detailSheet'
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
    </MarketingShell>
  );
};

export default Pricing;
