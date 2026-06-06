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

import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  ArrowRight,
  BadgeCheck,
  CalendarClock,
  CheckCircle2,
  CreditCard,
  Crown,
  LockKeyhole,
  Package,
  Sparkles,
  Wallet,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@douyinfe/semi-ui';
import { SiAlipay, SiStripe, SiWechat } from 'react-icons/si';
import MarketingShell from '../../components/marketing/MarketingShell';
import {
  API,
  getSystemName,
  renderQuota,
  renderQuotaWithAmount,
  showError,
  showSuccess,
  updateAPI,
} from '../../helpers';
import { getCurrencyConfig } from '../../helpers/render';
import {
  formatSubscriptionDuration,
  formatSubscriptionResetPeriod,
} from '../../helpers/subscriptionFormat';

const safeJsonParse = (value, fallback) => {
  try {
    if (value === null || value === undefined || value === '') {
      return fallback;
    }
    if (typeof value !== 'string') {
      return value;
    }
    return JSON.parse(value);
  } catch (error) {
    return fallback;
  }
};

const generatePresetAmounts = (minAmount) => {
  const base = Number(minAmount) > 0 ? Number(minAmount) : 1;
  return [1, 5, 10, 30, 50, 100].map((multiplier) => ({
    value: base * multiplier,
    discount: 1,
  }));
};

const getMethodColor = (type) => {
  if (type === 'alipay') return '#2d8cff';
  if (type === 'wxpay') return '#20c067';
  if (type === 'stripe') return '#635bff';
  if (type === 'creem') return '#37c2b2';
  if (typeof type === 'string' && type.startsWith('waffo:')) {
    return '#66d7ff';
  }
  return '#84efff';
};

const normalizePayMethods = (rawPayMethods = []) => {
  let payMethods = rawPayMethods;
  if (typeof payMethods === 'string') {
    payMethods = safeJsonParse(payMethods, []);
  }
  if (!Array.isArray(payMethods)) {
    return [];
  }
  return payMethods
    .filter((method) => method?.name && method?.type)
    .map((method) => {
      const normalizedMinTopup = Number(method.min_topup);
      return {
        ...method,
        color: method.color || getMethodColor(method.type),
        min_topup: Number.isFinite(normalizedMinTopup) ? normalizedMinTopup : 0,
      };
    });
};

const submitEpayForm = ({ url, params }) => {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';
  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) {
    form.target = '_blank';
  }
  Object.keys(params || {}).forEach((key) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = params[key];
    form.appendChild(input);
  });
  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
};

const Purchase = () => {
  const { t } = useTranslation();
  const systemName = getSystemName();
  const [user, setUser] = useState(() =>
    safeJsonParse(localStorage.getItem('user'), null),
  );
  const initialLoggedIn = Boolean(
    safeJsonParse(localStorage.getItem('user'), null)?.id,
  );
  const [purchaseMode, setPurchaseMode] = useState('topup');
  const [topupMode, setTopupMode] = useState('fixed');
  const [topupLoading, setTopupLoading] = useState(initialLoggedIn);
  const [subscriptionLoading, setSubscriptionLoading] = useState(initialLoggedIn);
  const [amountLoading, setAmountLoading] = useState(false);
  const [topupSubmitting, setTopupSubmitting] = useState(false);
  const [subscriptionSubmitting, setSubscriptionSubmitting] = useState(false);

  const [topupInfo, setTopupInfo] = useState({
    amount_options: [],
    discount: {},
  });
  const [presetAmounts, setPresetAmounts] = useState([]);
  const [selectedPreset, setSelectedPreset] = useState(null);
  const [topUpCount, setTopUpCount] = useState(0);
  const [minTopUp, setMinTopUp] = useState(1);
  const [amount, setAmount] = useState(0);

  const [regularPayMethods, setRegularPayMethods] = useState([]);
  const [waffoPayMethods, setWaffoPayMethods] = useState([]);
  const [selectedTopupMethod, setSelectedTopupMethod] = useState('');
  const [selectedSubscriptionMethod, setSelectedSubscriptionMethod] =
    useState('');

  const [enableOnlineTopUp, setEnableOnlineTopUp] = useState(false);
  const [enableStripeTopUp, setEnableStripeTopUp] = useState(false);
  const [enableCreemTopUp, setEnableCreemTopUp] = useState(false);
  const [enableWaffoTopUp, setEnableWaffoTopUp] = useState(false);
  const [waffoMinTopUp, setWaffoMinTopUp] = useState(1);
  const [creemProducts, setCreemProducts] = useState([]);

  const [subscriptionPlans, setSubscriptionPlans] = useState([]);
  const [selectedPlanId, setSelectedPlanId] = useState(null);
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [allSubscriptions, setAllSubscriptions] = useState([]);
  const [billingPreference, setBillingPreference] =
    useState('subscription_first');

  const isLoggedIn = Boolean(user?.id);
  const status = safeJsonParse(localStorage.getItem('status'), {});
  const topUpLink = status?.top_up_link || '';
  const { symbol, rate } = getCurrencyConfig();

  const resetPurchaseData = () => {
    setTopupLoading(false);
    setSubscriptionLoading(false);
    setAmountLoading(false);
    setTopupSubmitting(false);
    setSubscriptionSubmitting(false);
    setTopupInfo({ amount_options: [], discount: {} });
    setPresetAmounts([]);
    setSelectedPreset(null);
    setTopUpCount(0);
    setMinTopUp(1);
    setAmount(0);
    setRegularPayMethods([]);
    setWaffoPayMethods([]);
    setSelectedTopupMethod('');
    setSelectedSubscriptionMethod('');
    setEnableOnlineTopUp(false);
    setEnableStripeTopUp(false);
    setEnableCreemTopUp(false);
    setEnableWaffoTopUp(false);
    setWaffoMinTopUp(1);
    setCreemProducts([]);
    setSubscriptionPlans([]);
    setSelectedPlanId(null);
    setActiveSubscriptions([]);
    setAllSubscriptions([]);
    setBillingPreference('subscription_first');
  };

  const handleAuthFailure = () => {
    localStorage.removeItem('user');
    updateAPI();
    setUser(null);
    resetPurchaseData();
  };

  const getErrorMessage = (error, fallback) => {
    if (error?.response?.status === 401) {
      handleAuthFailure();
      return t('登录已过期，请重新登录');
    }
    const responseData = error?.response?.data;
    if (typeof responseData?.data === 'string' && responseData.data) {
      return responseData.data;
    }
    if (responseData?.message) {
      return responseData.message;
    }
    return error?.message || fallback;
  };

  const safeRenderQuota = (value) => {
    const text = renderQuota(Number(value || 0));
    return String(text).includes('NaN') ? `${Number(value || 0)}` : text;
  };

  const safeRenderQuotaAmount = (value) => {
    const text = renderQuotaWithAmount(Number(value || 0));
    return String(text).includes('NaN')
      ? `${Number(value || 0).toFixed(2)}`
      : text;
  };

  const waffoMethods = useMemo(() => {
    if (!enableWaffoTopUp) {
      return [];
    }
    return (waffoPayMethods || []).map((method, index) => ({
      ...method,
      type: `waffo:${index}`,
      min_topup: Number(waffoMinTopUp || 1),
      color: method.color || getMethodColor(`waffo:${index}`),
    }));
  }, [enableWaffoTopUp, waffoMinTopUp, waffoPayMethods]);

  const topupMethods = useMemo(() => {
    const methods = regularPayMethods.filter((method) => {
      if (method.type === 'creem') {
        return false;
      }
      if (method.type === 'stripe') {
        return enableStripeTopUp;
      }
      return enableOnlineTopUp;
    });
    return [...methods, ...waffoMethods];
  }, [
    enableOnlineTopUp,
    enableStripeTopUp,
    regularPayMethods,
    waffoMethods,
  ]);

  const selectedPlan = useMemo(() => {
    if (subscriptionPlans.length === 0) {
      return null;
    }
    return (
      subscriptionPlans.find(
        (item) => String(item?.plan?.id) === String(selectedPlanId),
      ) || subscriptionPlans[0]
    );
  }, [selectedPlanId, subscriptionPlans]);

  const subscriptionPlanCountMap = useMemo(() => {
    const map = new Map();
    (allSubscriptions || []).forEach((item) => {
      const planId = item?.subscription?.plan_id;
      if (!planId) return;
      map.set(planId, (map.get(planId) || 0) + 1);
    });
    return map;
  }, [allSubscriptions]);

  const subscriptionMethods = useMemo(() => {
    const methods = enableOnlineTopUp
      ? regularPayMethods.filter(
          (method) => method.type !== 'stripe' && method.type !== 'creem',
        )
      : [];
    const plan = selectedPlan?.plan;
    if (enableStripeTopUp && plan?.stripe_price_id) {
      methods.push({
        type: 'stripe',
        name: 'Stripe',
        color: getMethodColor('stripe'),
      });
    }
    if (enableCreemTopUp && plan?.creem_product_id) {
      methods.push({
        type: 'creem',
        name: 'Creem',
        color: getMethodColor('creem'),
      });
    }
    return methods;
  }, [
    enableCreemTopUp,
    enableOnlineTopUp,
    enableStripeTopUp,
    regularPayMethods,
    selectedPlan,
  ]);

  const topupDiscountRate = useMemo(() => {
    const discountMap = topupInfo?.discount || {};
    const currentDiscount =
      discountMap[topUpCount] ?? discountMap[String(topUpCount)] ?? 1;
    const numericDiscount = Number(currentDiscount);
    return Number.isFinite(numericDiscount) && numericDiscount > 0
      ? numericDiscount
      : 1;
  }, [topUpCount, topupInfo]);

  const originalTopupAmount =
    amount > 0 && topupDiscountRate > 0 ? amount / topupDiscountRate : 0;
  const topupSavedAmount =
    originalTopupAmount > amount ? originalTopupAmount - amount : 0;

  const selectedTopupMethodInfo = useMemo(
    () => topupMethods.find((method) => method.type === selectedTopupMethod),
    [selectedTopupMethod, topupMethods],
  );

  const selectedPlanPurchaseCount = selectedPlan?.plan?.id
    ? subscriptionPlanCountMap.get(selectedPlan.plan.id) || 0
    : 0;
  const planPurchaseLimit = Number(selectedPlan?.plan?.max_purchase_per_user || 0);
  const limitReached =
    planPurchaseLimit > 0 && selectedPlanPurchaseCount >= planPurchaseLimit;

  const planPrice = Number(selectedPlan?.plan?.price_amount || 0);
  const convertedPlanPrice = planPrice * rate;
  const displayPlanPrice = convertedPlanPrice.toFixed(
    Number.isInteger(convertedPlanPrice) ? 0 : 2,
  );

  const heroStats = [
    {
      label: t('可用支付方式'),
      value: isLoggedIn ? String(topupMethods.length || 0) : '4+',
    },
    {
      label: t('订阅套餐'),
      value: isLoggedIn ? String(subscriptionPlans.length || 0) : '实时',
    },
    {
      label: t('当前余额'),
      value: isLoggedIn ? safeRenderQuota(user?.quota || 0) : t('登录后可见'),
    },
  ];

  const getPaymentMinTopUp = (payment) => {
    const configuredMinTopUp = Number(
      topupMethods.find((method) => method.type === payment)?.min_topup,
    );
    return Number.isFinite(configuredMinTopUp) && configuredMinTopUp > 0
      ? configuredMinTopUp
      : minTopUp;
  };

  const requestTopupAmount = async (payment, value) => {
    if (!payment || !value || Number(value) <= 0) {
      setAmount(0);
      return;
    }
    setAmountLoading(true);
    try {
      let path = '/api/user/amount';
      if (payment === 'stripe') {
        path = '/api/user/stripe/amount';
      } else if (payment.startsWith('waffo:')) {
        path = '/api/user/waffo/amount';
      }
      const res = await API.post(
        path,
        {
          amount: payment.startsWith('waffo:')
            ? parseInt(value)
            : parseFloat(value),
        },
        { skipErrorHandler: true },
      );
      if (res.data?.message === 'success') {
        setAmount(Number(res.data.data || 0));
        return;
      }
      setAmount(0);
    } catch (error) {
      if (error?.response?.status === 401) {
        handleAuthFailure();
        return;
      }
      setAmount(0);
    } finally {
      setAmountLoading(false);
    }
  };

  const fetchTopupInfo = async () => {
    setTopupLoading(true);
    try {
      const res = await API.get('/api/user/topup/info', {
        skipErrorHandler: true,
      });
      const { success, data, message } = res.data || {};
      if (!success) {
        showError(message || t('获取充值配置失败'));
        return;
      }

      const normalizedPayMethods = normalizePayMethods(data?.pay_methods || []);
      const nextEnableStripeTopUp = Boolean(data?.enable_stripe_topup);
      const nextEnableOnlineTopUp = Boolean(data?.enable_online_topup);
      const nextEnableCreemTopUp = Boolean(data?.enable_creem_topup);
      const nextEnableWaffoTopUp = Boolean(data?.enable_waffo_topup);
      const nextMinTopup = Number(
        data?.min_topup ||
          data?.stripe_min_topup ||
          data?.waffo_min_topup ||
          1,
      );
      const nextPresets =
        Array.isArray(data?.amount_options) && data.amount_options.length > 0
          ? data.amount_options.map((item) => ({
              value: Number(item),
              discount: Number(data?.discount?.[item] || 1),
            }))
          : generatePresetAmounts(nextMinTopup);
      const defaultAmount = Number(nextPresets[0]?.value || nextMinTopup || 1);

      setTopupInfo({
        amount_options: data?.amount_options || [],
        discount: data?.discount || {},
      });
      setRegularPayMethods(normalizedPayMethods);
      setWaffoPayMethods(data?.waffo_pay_methods || []);
      setEnableStripeTopUp(nextEnableStripeTopUp);
      setEnableOnlineTopUp(nextEnableOnlineTopUp);
      setEnableCreemTopUp(nextEnableCreemTopUp);
      setEnableWaffoTopUp(nextEnableWaffoTopUp);
      setWaffoMinTopUp(Number(data?.waffo_min_topup || 1));
      setMinTopUp(nextMinTopup);
      setPresetAmounts(nextPresets);
      setSelectedPreset(defaultAmount);
      setTopUpCount(defaultAmount);
      setCreemProducts(safeJsonParse(data?.creem_products, []));
    } catch (error) {
      if (error?.response?.status === 401) {
        handleAuthFailure();
        return;
      }
      showError(getErrorMessage(error, t('获取充值配置异常')));
    } finally {
      setTopupLoading(false);
    }
  };

  const fetchSubscriptionPlans = async () => {
    setSubscriptionLoading(true);
    try {
      const res = await API.get('/api/subscription/plans', {
        skipErrorHandler: true,
      });
      if (res.data?.success) {
        setSubscriptionPlans(res.data.data || []);
      } else {
        setSubscriptionPlans([]);
      }
    } catch (error) {
      if (error?.response?.status === 401) {
        handleAuthFailure();
        return;
      }
      setSubscriptionPlans([]);
    } finally {
      setSubscriptionLoading(false);
    }
  };

  const fetchSubscriptionSelf = async () => {
    try {
      const res = await API.get('/api/subscription/self', {
        skipErrorHandler: true,
      });
      if (res.data?.success) {
        setBillingPreference(
          res.data.data?.billing_preference || 'subscription_first',
        );
        setActiveSubscriptions(res.data.data?.subscriptions || []);
        setAllSubscriptions(res.data.data?.all_subscriptions || []);
      }
    } catch (error) {
      if (error?.response?.status === 401) {
        handleAuthFailure();
      }
    }
  };

  useEffect(() => {
    if (!isLoggedIn) {
      return;
    }
    fetchTopupInfo().catch(() => {});
    fetchSubscriptionPlans().catch(() => {});
    fetchSubscriptionSelf().catch(() => {});
  }, [isLoggedIn]);

  useEffect(() => {
    if (topupMethods.length === 0) {
      setSelectedTopupMethod('');
      return;
    }
    if (!topupMethods.some((method) => method.type === selectedTopupMethod)) {
      setSelectedTopupMethod(topupMethods[0].type);
    }
  }, [selectedTopupMethod, topupMethods]);

  useEffect(() => {
    if (subscriptionPlans.length === 0) {
      setSelectedPlanId(null);
      return;
    }
    if (
      !subscriptionPlans.some(
        (item) => String(item?.plan?.id) === String(selectedPlanId),
      )
    ) {
      setSelectedPlanId(subscriptionPlans[0]?.plan?.id || null);
    }
  }, [selectedPlanId, subscriptionPlans]);

  useEffect(() => {
    if (subscriptionMethods.length === 0) {
      setSelectedSubscriptionMethod('');
      return;
    }
    if (
      !subscriptionMethods.some(
        (method) => method.type === selectedSubscriptionMethod,
      )
    ) {
      setSelectedSubscriptionMethod(subscriptionMethods[0].type);
    }
  }, [selectedSubscriptionMethod, subscriptionMethods]);

  useEffect(() => {
    if (!isLoggedIn || purchaseMode !== 'topup') {
      return;
    }
    if (!selectedTopupMethod || !topUpCount || Number(topUpCount) <= 0) {
      setAmount(0);
      return;
    }
    const timer = window.setTimeout(() => {
      requestTopupAmount(selectedTopupMethod, topUpCount).catch(() => {});
    }, 180);
    return () => window.clearTimeout(timer);
  }, [isLoggedIn, purchaseMode, selectedTopupMethod, topUpCount]);

  const onSelectPreset = (preset) => {
    setTopupMode('fixed');
    setSelectedPreset(preset.value);
    setTopUpCount(Number(preset.value || 0));
  };

  const onCustomAmountChange = (event) => {
    const nextValue = Number(event.target.value || 0);
    setTopupMode('custom');
    setSelectedPreset(null);
    setTopUpCount(nextValue);
  };

  const triggerTopup = async () => {
    if (!isLoggedIn) {
      return;
    }
    if (!selectedTopupMethod) {
      showError(t('请选择支付方式'));
      return;
    }
    const minAllowed = getPaymentMinTopUp(selectedTopupMethod);
    if (Number(topUpCount) < Number(minAllowed)) {
      showError(`${t('充值数量不能小于')}${minAllowed}`);
      return;
    }

    setTopupSubmitting(true);
    try {
      if (selectedTopupMethod === 'stripe') {
        const res = await API.post(
          '/api/user/stripe/pay',
          {
            amount: parseInt(topUpCount),
            payment_method: 'stripe',
          },
          { skipErrorHandler: true },
        );
        if (res.data?.message === 'success') {
          window.open(res.data.data?.pay_link, '_blank');
          showSuccess(t('已打开支付页面'));
          return;
        }
        showError(res.data?.data || res.data?.message || t('支付失败'));
        return;
      }

      if (selectedTopupMethod.startsWith('waffo:')) {
        const payMethodIndex = Number(selectedTopupMethod.split(':')[1]);
        const res = await API.post(
          '/api/user/waffo/pay',
          {
            amount: parseInt(topUpCount),
            pay_method_index: Number.isFinite(payMethodIndex)
              ? payMethodIndex
              : 0,
          },
          { skipErrorHandler: true },
        );
        if (res.data?.message === 'success' && res.data?.data?.payment_url) {
          window.open(res.data.data.payment_url, '_blank');
          showSuccess(t('已打开支付页面'));
          return;
        }
        showError(res.data?.data || res.data?.message || t('支付失败'));
        return;
      }

      const res = await API.post(
        '/api/user/pay',
        {
          amount: parseInt(topUpCount),
          payment_method: selectedTopupMethod,
        },
        { skipErrorHandler: true },
      );
      if (res.data?.message === 'success') {
        submitEpayForm({ url: res.data.url, params: res.data.data });
        showSuccess(t('已发起支付'));
        return;
      }
      showError(res.data?.data || res.data?.message || t('支付失败'));
    } catch (error) {
      showError(getErrorMessage(error, t('支付请求失败')));
    } finally {
      setTopupSubmitting(false);
    }
  };

  const triggerCreemTopup = async (product) => {
    if (!isLoggedIn) {
      return;
    }
    if (!product?.productId) {
      showError(t('产品配置错误，请联系管理员'));
      return;
    }
    setTopupSubmitting(true);
    try {
      const res = await API.post(
        '/api/user/creem/pay',
        {
          product_id: product.productId,
          payment_method: 'creem',
        },
        { skipErrorHandler: true },
      );
      if (res.data?.message === 'success') {
        window.open(res.data.data?.checkout_url, '_blank');
        showSuccess(t('已打开支付页面'));
        return;
      }
      showError(res.data?.data || res.data?.message || t('支付失败'));
    } catch (error) {
      showError(getErrorMessage(error, t('支付请求失败')));
    } finally {
      setTopupSubmitting(false);
    }
  };

  const triggerSubscriptionPurchase = async () => {
    if (!isLoggedIn) {
      return;
    }
    if (!selectedPlan?.plan?.id) {
      showError(t('请选择订阅套餐'));
      return;
    }
    if (!selectedSubscriptionMethod) {
      showError(t('请选择支付方式'));
      return;
    }
    if (limitReached) {
      showError(
        `${t('已达到购买上限')} (${selectedPlanPurchaseCount}/${planPurchaseLimit})`,
      );
      return;
    }

    setSubscriptionSubmitting(true);
    try {
      if (selectedSubscriptionMethod === 'stripe') {
        const res = await API.post(
          '/api/subscription/stripe/pay',
          { plan_id: selectedPlan.plan.id },
          { skipErrorHandler: true },
        );
        if (res.data?.message === 'success') {
          window.open(res.data.data?.pay_link, '_blank');
          showSuccess(t('已打开支付页面'));
          return;
        }
        showError(res.data?.data || res.data?.message || t('支付失败'));
        return;
      }

      if (selectedSubscriptionMethod === 'creem') {
        const res = await API.post(
          '/api/subscription/creem/pay',
          { plan_id: selectedPlan.plan.id },
          { skipErrorHandler: true },
        );
        if (res.data?.message === 'success') {
          window.open(res.data.data?.checkout_url, '_blank');
          showSuccess(t('已打开支付页面'));
          return;
        }
        showError(res.data?.data || res.data?.message || t('支付失败'));
        return;
      }

      const res = await API.post(
        '/api/subscription/epay/pay',
        {
          plan_id: selectedPlan.plan.id,
          payment_method: selectedSubscriptionMethod,
        },
        { skipErrorHandler: true },
      );
      if (res.data?.message === 'success') {
        submitEpayForm({ url: res.data.url, params: res.data.data });
        showSuccess(t('已发起支付'));
        return;
      }
      showError(res.data?.data || res.data?.message || t('支付失败'));
    } catch (error) {
      showError(getErrorMessage(error, t('支付请求失败')));
    } finally {
      setSubscriptionSubmitting(false);
    }
  };

  const renderMethodIcon = (method) => {
    if (method?.type === 'alipay') {
      return <SiAlipay size={16} color='#2d8cff' />;
    }
    if (method?.type === 'wxpay') {
      return <SiWechat size={16} color='#20c067' />;
    }
    if (method?.type === 'stripe') {
      return <SiStripe size={16} color='#635bff' />;
    }
    if (method?.icon) {
      return (
        <img
          src={method.icon}
          alt={method.name}
          className='landing-purchase__methodLogo'
        />
      );
    }
    return <CreditCard size={16} color={method?.color || '#84efff'} />;
  };

  const renderLoginPrompt = () => (
    <div className='landing-purchase__lockedState'>
      <span className='landing-purchase__lockedIcon'>
        <LockKeyhole size={16} />
      </span>
      <div>
        <h3>{t('登录后继续购买')}</h3>
        <p>
          {t(
            '充值档位、支付方式和订阅套餐来自当前账户的可用配置，登录后即可查看真实价格并直接下单。',
          )}
        </p>
      </div>
      <div className='landing-purchase__lockedActions'>
        <Link to='/login' className='landing-purchase__primaryLink'>
          {t('前往登录')}
        </Link>
        <Link to='/register' className='landing-purchase__secondaryLink'>
          {t('注册账户')}
        </Link>
      </div>
    </div>
  );

  const topupLeftContent = isLoggedIn ? (
    <>
      <div className='landing-purchase__sectionCard'>
        <div className='landing-purchase__sectionHead'>
          <div>
            <span className='landing-purchase__sectionEyebrow'>
              {t('充值方式')}
            </span>
            <h3>{t('选择充值金额')}</h3>
          </div>
          <div className='landing-purchase__segment'>
            <button
              type='button'
              className={[
                'landing-purchase__segmentButton',
                topupMode === 'fixed'
                  ? 'landing-purchase__segmentButton--active'
                  : '',
              ]
                .filter(Boolean)
                .join(' ')}
              onClick={() => setTopupMode('fixed')}
            >
              {t('固定金额')}
            </button>
            <button
              type='button'
              className={[
                'landing-purchase__segmentButton',
                topupMode === 'custom'
                  ? 'landing-purchase__segmentButton--active'
                  : '',
              ]
                .filter(Boolean)
                .join(' ')}
              onClick={() => {
                setTopupMode('custom');
                setSelectedPreset(null);
              }}
            >
              {t('自定义金额')}
            </button>
          </div>
        </div>

        {topupLoading ? (
          <div className='landing-purchase__ghostGrid'>
            {[1, 2, 3, 4, 5, 6].map((item) => (
              <div key={item} className='landing-purchase__ghostCard' />
            ))}
          </div>
        ) : (
          <>
            {topupMode === 'fixed' ? (
              <div className='landing-purchase__amountGrid'>
                {presetAmounts.map((preset) => {
                  const isActive = Number(selectedPreset) === Number(preset.value);
                  const discount = Number(preset.discount || 1);
                  return (
                    <button
                      type='button'
                      key={preset.value}
                      className={[
                        'landing-purchase__amountCard',
                        isActive ? 'landing-purchase__amountCard--active' : '',
                      ]
                        .filter(Boolean)
                        .join(' ')}
                      onClick={() => onSelectPreset(preset)}
                    >
                      <span className='landing-purchase__amountValue'>
                        {safeRenderQuotaAmount(preset.value)}
                      </span>
                      <span className='landing-purchase__amountMeta'>
                        {discount < 1
                          ? `${t('限时')} ${Math.round(discount * 100)}%`
                          : t('即时到账')}
                      </span>
                    </button>
                  );
                })}
              </div>
            ) : (
              <div className='landing-purchase__customCard'>
                <label
                  className='landing-purchase__fieldLabel'
                  htmlFor='purchase-custom-amount'
                >
                  {t('输入充值数量')}
                </label>
                <input
                  id='purchase-custom-amount'
                  className='landing-purchase__input'
                  type='number'
                  min={minTopUp}
                  step='1'
                  value={topUpCount || ''}
                  onChange={onCustomAmountChange}
                  placeholder={`${t('最低')} ${safeRenderQuotaAmount(minTopUp)}`}
                />
                <p className='landing-purchase__helperText'>
                  {t('当前支付方式最低充值')} {safeRenderQuotaAmount(
                    getPaymentMinTopUp(selectedTopupMethod || ''),
                  )}
                </p>
              </div>
            )}
          </>
        )}
      </div>

      {enableCreemTopUp && creemProducts.length > 0 && (
        <div className='landing-purchase__sectionCard'>
          <div className='landing-purchase__sectionHead'>
            <div>
              <span className='landing-purchase__sectionEyebrow'>
                Creem
              </span>
              <h3>{t('产品充值')}</h3>
            </div>
          </div>
          <div className='landing-purchase__creemGrid'>
            {creemProducts.map((product) => (
              <article
                key={product.productId || product.name}
                className='landing-purchase__creemCard'
              >
                <div>
                  <h4>{product.name || t('充值产品')}</h4>
                  <p>{safeRenderQuotaAmount(product.quota || 0)}</p>
                </div>
                <div className='landing-purchase__creemFooter'>
                  <span>
                    {(product.currency || 'USD') === 'EUR' ? '€' : '$'}
                    {product.price}
                  </span>
                  <Button
                    theme='solid'
                    type='primary'
                    loading={topupSubmitting}
                    onClick={() => triggerCreemTopup(product)}
                  >
                    {t('立即购买')}
                  </Button>
                </div>
              </article>
            ))}
          </div>
        </div>
      )}

      {!topupLoading &&
        topupMethods.length === 0 &&
        creemProducts.length === 0 &&
        topUpLink && (
          <div className='landing-purchase__sectionCard landing-purchase__fallbackCard'>
            <div>
              <span className='landing-purchase__sectionEyebrow'>
                {t('外部购买')}
              </span>
              <h3>{t('当前站点使用外部充值入口')}</h3>
              <p>
                {t(
                  '当前没有可直接在页面内发起的支付方式，你仍然可以通过管理员配置的充值链接继续购买。',
                )}
              </p>
            </div>
            <a
              href={topUpLink}
              target='_blank'
              rel='noreferrer'
              className='landing-purchase__primaryLink'
            >
              {t('前往购买')}
            </a>
          </div>
        )}
    </>
  ) : (
    <>
      <div className='landing-purchase__sectionCard'>
        <div className='landing-purchase__sectionHead'>
          <div>
            <span className='landing-purchase__sectionEyebrow'>
              {t('充值方式')}
            </span>
            <h3>{t('选择充值金额')}</h3>
          </div>
          <div className='landing-purchase__segment'>
            <span className='landing-purchase__segmentButton landing-purchase__segmentButton--active'>
              {t('固定金额')}
            </span>
            <span className='landing-purchase__segmentButton'>
              {t('自定义金额')}
            </span>
          </div>
        </div>
        <div className='landing-purchase__ghostGrid'>
          {[1, 2, 3, 4, 5, 6].map((item) => (
            <div key={item} className='landing-purchase__ghostCard' />
          ))}
        </div>
      </div>
      {renderLoginPrompt()}
    </>
  );

  const subscriptionLeftContent = isLoggedIn ? (
    <>
      {activeSubscriptions.length > 0 && (
        <div className='landing-purchase__noticeCard'>
          <div className='landing-purchase__noticeIcon'>
            <BadgeCheck size={16} />
          </div>
          <div>
            <strong>{t('你当前已有生效订阅')}</strong>
            <p>
              {t('当前生效订阅数量')} {activeSubscriptions.length} ·{' '}
              {t('计费偏好')} {billingPreference}
            </p>
          </div>
        </div>
      )}

      <div className='landing-purchase__planGrid'>
        {subscriptionLoading ? (
          [1, 2, 3].map((item) => (
            <div key={item} className='landing-purchase__planGhost' />
          ))
        ) : subscriptionPlans.length > 0 ? (
          subscriptionPlans.map((item, index) => {
            const plan = item?.plan;
            const totalAmount = Number(plan?.total_amount || 0);
            const priceAmount = Number(plan?.price_amount || 0);
            const convertedPrice = priceAmount * rate;
            const purchaseCount = subscriptionPlanCountMap.get(plan?.id) || 0;
            const limit = Number(plan?.max_purchase_per_user || 0);
            const reached = limit > 0 && purchaseCount >= limit;
            const isActive = String(selectedPlanId) === String(plan?.id);

            return (
              <article
                key={plan?.id || index}
                className={[
                  'landing-purchase__planCard',
                  isActive ? 'landing-purchase__planCard--active' : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
                onClick={() => setSelectedPlanId(plan?.id)}
                role='button'
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    setSelectedPlanId(plan?.id);
                  }
                }}
              >
                <div className='landing-purchase__planHead'>
                  <div>
                    <div className='landing-purchase__planTitleRow'>
                      <h3>{plan?.title || t('订阅套餐')}</h3>
                      {index === 0 && subscriptionPlans.length > 1 && (
                        <span className='landing-purchase__planBadge'>
                          <Sparkles size={12} />
                          {t('推荐')}
                        </span>
                      )}
                    </div>
                    {plan?.subtitle && (
                      <p className='landing-purchase__planSubtitle'>
                        {plan.subtitle}
                      </p>
                    )}
                  </div>
                  <div className='landing-purchase__planPrice'>
                    <span>{symbol}</span>
                    <strong>
                      {convertedPrice.toFixed(
                        Number.isInteger(convertedPrice) ? 0 : 2,
                      )}
                    </strong>
                  </div>
                </div>

                <div className='landing-purchase__planMetaList'>
                  <span>
                    <CalendarClock size={14} />
                    {formatSubscriptionDuration(plan, t)}
                  </span>
                  <span>
                    <Package size={14} />
                    {totalAmount > 0
                      ? safeRenderQuota(totalAmount)
                      : t('不限额度')}
                  </span>
                  <span>
                    <CheckCircle2 size={14} />
                    {formatSubscriptionResetPeriod(plan, t)}
                  </span>
                  {plan?.upgrade_group ? (
                    <span>
                      <Crown size={14} />
                      {plan.upgrade_group}
                    </span>
                  ) : null}
                </div>

                {limit > 0 && (
                  <div className='landing-purchase__planLimit'>
                    {t('已购')} {purchaseCount}/{limit}
                    {reached ? ` · ${t('已达上限')}` : ''}
                  </div>
                )}
              </article>
            );
          })
        ) : (
          <div className='landing-purchase__sectionCard landing-purchase__emptyCard'>
            <h3>{t('暂无可购买套餐')}</h3>
            <p>{t('管理员还没有发布可购买的订阅套餐。')}</p>
          </div>
        )}
      </div>
    </>
  ) : (
    <>
      <div className='landing-purchase__planGrid'>
        {[1, 2, 3].map((item) => (
          <div key={item} className='landing-purchase__planGhost' />
        ))}
      </div>
      {renderLoginPrompt()}
    </>
  );

  return (
    <MarketingShell activeNav='buy' className='landing-purchase'>
      <section className='landing-purchase__hero'>
        <div className='landing-purchase__heroGrid'>
          <div className='landing-purchase__heroContent'>
            <p className='landing-purchase__eyebrow'>{t('灵活购买')}</p>
            <h1 className='landing-purchase__title'>
              {t('按需充值，或直接订阅')}
              <br />
              <span>{systemName}</span>
              {t(' 当前可用模型权益')}
            </h1>
            <p className='landing-purchase__description'>
              {t(
                '在一个页面里完成充值和订阅选择。按量充值适合弹性消耗，订阅套餐适合长期稳定调用，所有可用档位与支付方式都直接读取现有接口。',
              )}
            </p>

            <div className='landing-purchase__heroActions'>
              <div className='landing-purchase__heroSignal'>
                <span>{t('实时价格')}</span>
                <span>{t('多支付方式')}</span>
                <span>{t('订阅权益')}</span>
              </div>
              {!isLoggedIn && (
                <Link to='/login' className='landing-purchase__primaryLink'>
                  {t('登录后购买')}
                  <ArrowRight size={16} />
                </Link>
              )}
            </div>
          </div>

          <div className='landing-purchase__heroPanel'>
            <div className='landing-purchase__heroPanelHeader'>
              <span className='landing-purchase__heroPanelTag'>
                {t('购买总览')}
              </span>
              <Wallet size={16} />
            </div>
            <div className='landing-purchase__heroStats'>
              {heroStats.map((stat) => (
                <div key={stat.label} className='landing-purchase__heroStat'>
                  <strong>{stat.value}</strong>
                  <span>{stat.label}</span>
                </div>
              ))}
            </div>
            <div className='landing-purchase__heroPanelNote'>
              <Sparkles size={15} />
              <span>
                {isLoggedIn
                  ? t('已登录状态下显示真实可购配置')
                  : t('未登录时展示购买结构，登录后加载真实数据')}
              </span>
            </div>
          </div>
        </div>
      </section>

      <section className='landing-purchase__workspace'>
        <div className='landing-purchase__modeSwitch'>
          <button
            type='button'
            className={[
              'landing-purchase__modeCard',
              purchaseMode === 'topup'
                ? 'landing-purchase__modeCard--active'
                : '',
            ]
              .filter(Boolean)
              .join(' ')}
            onClick={() => setPurchaseMode('topup')}
          >
            <div>
              <span>{t('随用随充')}</span>
              <strong>{t('按量充值')}</strong>
              <p>{t('适合灵活测试、项目冲量与短期消耗场景')}</p>
            </div>
            <CreditCard size={18} />
          </button>
          <button
            type='button'
            className={[
              'landing-purchase__modeCard',
              purchaseMode === 'subscription'
                ? 'landing-purchase__modeCard--active'
                : '',
            ]
              .filter(Boolean)
              .join(' ')}
            onClick={() => setPurchaseMode('subscription')}
          >
            <div>
              <span>{t('订阅套餐')}</span>
              <strong>{t('稳定权益')}</strong>
              <p>{t('适合长期使用、固定预算和需要持续额度的团队')}</p>
            </div>
            <Crown size={18} />
          </button>
        </div>

        <div className='landing-purchase__contentGrid'>
          <div className='landing-purchase__left'>
            {purchaseMode === 'topup'
              ? topupLeftContent
              : subscriptionLeftContent}
          </div>

          <aside className='landing-purchase__right'>
            {purchaseMode === 'topup' ? (
              <div className='landing-purchase__summaryCard'>
                <div className='landing-purchase__summaryHead'>
                  <div>
                    <span className='landing-purchase__sectionEyebrow'>
                      {t('订单确认')}
                    </span>
                    <h3>{t('充值明细')}</h3>
                  </div>
                  <ReceiptBadge />
                </div>

                <div className='landing-purchase__summaryRows'>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('充值数量')}</span>
                    <strong>
                      {topUpCount > 0 ? safeRenderQuotaAmount(topUpCount) : '--'}
                    </strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('支付金额')}</span>
                    <strong>
                      {isLoggedIn
                        ? amountLoading
                          ? t('计算中...')
                          : amount > 0
                            ? `${amount.toFixed(2)} ${t('元')}`
                            : '--'
                        : t('登录后显示')}
                    </strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('优惠节省')}</span>
                    <strong>
                      {isLoggedIn && topupSavedAmount > 0
                        ? `${topupSavedAmount.toFixed(2)} ${t('元')}`
                        : '--'}
                    </strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('最低充值')}</span>
                    <strong>
                      {safeRenderQuotaAmount(
                        getPaymentMinTopUp(selectedTopupMethod || ''),
                      )}
                    </strong>
                  </div>
                </div>

                <div className='landing-purchase__methodSection'>
                  <h4>{t('支付方式')}</h4>
                  <div className='landing-purchase__methodGrid'>
                    {isLoggedIn && topupMethods.length > 0 ? (
                      topupMethods.map((method) => (
                        <button
                          type='button'
                          key={method.type}
                          className={[
                            'landing-purchase__methodButton',
                            selectedTopupMethod === method.type
                              ? 'landing-purchase__methodButton--active'
                              : '',
                          ]
                            .filter(Boolean)
                            .join(' ')}
                          onClick={() => setSelectedTopupMethod(method.type)}
                        >
                          <span className='landing-purchase__methodIcon'>
                            {renderMethodIcon(method)}
                          </span>
                          <span>{method.name}</span>
                        </button>
                      ))
                    ) : (
                      <div className='landing-purchase__emptyHint'>
                        {isLoggedIn
                          ? t('当前没有可用的页面内支付方式')
                          : t('登录后查看可用支付方式')}
                      </div>
                    )}
                  </div>
                </div>

                {selectedTopupMethodInfo?.description && (
                  <p className='landing-purchase__helperText'>
                    {selectedTopupMethodInfo.description}
                  </p>
                )}

                {isLoggedIn ? (
                  <Button
                    theme='solid'
                    type='primary'
                    size='large'
                    className='landing-purchase__actionButton'
                    loading={topupSubmitting}
                    disabled={
                      topupMethods.length === 0 ||
                      !selectedTopupMethod ||
                      Number(topUpCount) <= 0
                    }
                    onClick={triggerTopup}
                  >
                    {t('立即充值')}
                  </Button>
                ) : (
                  <Link to='/login' className='landing-purchase__ctaLink'>
                    {t('登录后继续购买')}
                  </Link>
                )}
              </div>
            ) : (
              <div className='landing-purchase__summaryCard'>
                <div className='landing-purchase__summaryHead'>
                  <div>
                    <span className='landing-purchase__sectionEyebrow'>
                      {t('订阅确认')}
                    </span>
                    <h3>{t('套餐明细')}</h3>
                  </div>
                  <Crown size={18} />
                </div>

                <div className='landing-purchase__summaryRows'>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('套餐名称')}</span>
                    <strong>{selectedPlan?.plan?.title || '--'}</strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('订阅时长')}</span>
                    <strong>
                      {selectedPlan?.plan
                        ? formatSubscriptionDuration(selectedPlan.plan, t)
                        : '--'}
                    </strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('额度重置')}</span>
                    <strong>
                      {selectedPlan?.plan
                        ? formatSubscriptionResetPeriod(selectedPlan.plan, t)
                        : '--'}
                    </strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('总额度')}</span>
                    <strong>
                      {selectedPlan?.plan
                        ? Number(selectedPlan.plan.total_amount || 0) > 0
                          ? safeRenderQuota(selectedPlan.plan.total_amount)
                          : t('不限额度')
                        : '--'}
                    </strong>
                  </div>
                  <div className='landing-purchase__summaryRow'>
                    <span>{t('应付金额')}</span>
                    <strong>
                      {selectedPlan?.plan
                        ? `${symbol}${displayPlanPrice}`
                        : '--'}
                    </strong>
                  </div>
                </div>

                {planPurchaseLimit > 0 && (
                  <div className='landing-purchase__limitNotice'>
                    {t('购买上限')} {selectedPlanPurchaseCount}/{planPurchaseLimit}
                  </div>
                )}

                <div className='landing-purchase__methodSection'>
                  <h4>{t('支付方式')}</h4>
                  <div className='landing-purchase__methodGrid'>
                    {isLoggedIn && subscriptionMethods.length > 0 ? (
                      subscriptionMethods.map((method) => (
                        <button
                          type='button'
                          key={method.type}
                          className={[
                            'landing-purchase__methodButton',
                            selectedSubscriptionMethod === method.type
                              ? 'landing-purchase__methodButton--active'
                              : '',
                          ]
                            .filter(Boolean)
                            .join(' ')}
                          onClick={() =>
                            setSelectedSubscriptionMethod(method.type)
                          }
                        >
                          <span className='landing-purchase__methodIcon'>
                            {renderMethodIcon(method)}
                          </span>
                          <span>{method.name}</span>
                        </button>
                      ))
                    ) : (
                      <div className='landing-purchase__emptyHint'>
                        {isLoggedIn
                          ? t('该套餐暂无可用支付方式')
                          : t('登录后查看可用支付方式')}
                      </div>
                    )}
                  </div>
                </div>

                {selectedPlan?.plan?.upgrade_group && (
                  <p className='landing-purchase__helperText'>
                    {t('购买成功后将升级分组至')} {selectedPlan.plan.upgrade_group}
                  </p>
                )}

                {isLoggedIn ? (
                  <Button
                    theme='solid'
                    type='primary'
                    size='large'
                    className='landing-purchase__actionButton'
                    loading={subscriptionSubmitting}
                    disabled={
                      !selectedPlan?.plan?.id ||
                      !selectedSubscriptionMethod ||
                      limitReached
                    }
                    onClick={triggerSubscriptionPurchase}
                  >
                    {limitReached ? t('已达购买上限') : t('立即订阅')}
                  </Button>
                ) : (
                  <Link to='/login' className='landing-purchase__ctaLink'>
                    {t('登录后继续购买')}
                  </Link>
                )}
              </div>
            )}
          </aside>
        </div>
      </section>
    </MarketingShell>
  );
};

const ReceiptBadge = () => (
  <span className='landing-purchase__receiptBadge'>
    <CheckCircle2 size={14} />
  </span>
);

export default Purchase;
