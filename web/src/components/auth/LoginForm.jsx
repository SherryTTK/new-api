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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Checkbox, Divider, Icon, Input, Modal } from '@douyinfe/semi-ui';
import {
  IconGithubLogo,
  IconKey,
  IconLock,
  IconMail,
} from '@douyinfe/semi-icons';
import {
  ArrowRight,
  Fingerprint,
  Orbit,
  ShieldCheck,
  Sparkles,
  Waypoints,
} from 'lucide-react';
import { SiDiscord } from 'react-icons/si';
import Turnstile from 'react-turnstile';
import TelegramLoginButton from 'react-telegram-login';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import {
  API,
  buildAssertionResult,
  getOAuthProviderIcon,
  getSystemName,
  isPasskeySupported,
  onCustomOAuthClicked,
  onDiscordOAuthClicked,
  onGitHubOAuthClicked,
  onLinuxDOOAuthClicked,
  onOIDCClicked,
  prepareCredentialRequestOptions,
  setUserData,
  showError,
  showInfo,
  showSuccess,
  updateAPI,
} from '../../helpers';
import OIDCIcon from '../common/logo/OIDCIcon';
import WeChatIcon from '../common/logo/WeChatIcon';
import LinuxDoIcon from '../common/logo/LinuxDoIcon';
import TwoFAVerification from './TwoFAVerification';
import MarketingShell from '../marketing/MarketingShell';

const LoginForm = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const githubButtonTextKeyByState = {
    idle: '使用 GitHub 继续',
    redirecting: '正在跳转 GitHub...',
    timeout: '请求超时，请刷新页面后重新发起 GitHub 登录',
  };
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
    wechat_verification_code: '',
  });
  const { username, password } = inputs;
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);
  const [wechatLoading, setWechatLoading] = useState(false);
  const [githubLoading, setGithubLoading] = useState(false);
  const [discordLoading, setDiscordLoading] = useState(false);
  const [oidcLoading, setOidcLoading] = useState(false);
  const [linuxdoLoading, setLinuxdoLoading] = useState(false);
  const [loginLoading, setLoginLoading] = useState(false);
  const [resetPasswordLoading, setResetPasswordLoading] = useState(false);
  const [wechatCodeSubmitLoading, setWechatCodeSubmitLoading] = useState(false);
  const [showTwoFA, setShowTwoFA] = useState(false);
  const [passkeySupported, setPasskeySupported] = useState(false);
  const [passkeyLoading, setPasskeyLoading] = useState(false);
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [hasUserAgreement, setHasUserAgreement] = useState(false);
  const [hasPrivacyPolicy, setHasPrivacyPolicy] = useState(false);
  const [githubButtonState, setGithubButtonState] = useState('idle');
  const [githubButtonDisabled, setGithubButtonDisabled] = useState(false);
  const [customOAuthLoading, setCustomOAuthLoading] = useState({});
  const githubTimeoutRef = useRef(null);
  const githubButtonText = t(githubButtonTextKeyByState[githubButtonState]);
  const systemName = getSystemName();
  const requiresAgreement = hasUserAgreement || hasPrivacyPolicy;

  const featureCards = [
    {
      icon: Orbit,
      title: t('统一聚合入口'),
      description: t('一个账户即可管理多家模型服务与多种调用场景。'),
    },
    {
      icon: ShieldCheck,
      title: t('认证与安全并重'),
      description: t('支持密码、Passkey、2FA 与第三方授权登录。'),
    },
    {
      icon: Waypoints,
      title: t('快速开始调用'),
      description: t('登录后即可进入控制台、创建令牌并查看模型价格。'),
    },
  ];

  const status = useMemo(() => {
    if (statusState?.status) return statusState.status;
    const savedStatus = localStorage.getItem('status');
    if (!savedStatus) return {};
    try {
      return JSON.parse(savedStatus) || {};
    } catch (err) {
      return {};
    }
  }, [statusState?.status]);

  const hasCustomOAuthProviders =
    (status.custom_oauth_providers || []).length > 0;
  const hasOAuthLoginOptions = Boolean(
    status.github_oauth ||
      status.discord_oauth ||
      status.oidc_enabled ||
      status.wechat_login ||
      status.linuxdo_oauth ||
      status.telegram_oauth ||
      hasCustomOAuthProviders,
  );
  const hasPasskeyLogin = Boolean(status.passkey_login && passkeySupported);
  const hasAlternativeLoginOptions = hasOAuthLoginOptions || hasPasskeyLogin;

  const affCode = new URLSearchParams(window.location.search).get('aff');
  if (affCode) {
    localStorage.setItem('aff', affCode);
  }

  useEffect(() => {
    if (status?.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(status.turnstile_site_key);
    }

    setHasUserAgreement(status?.user_agreement_enabled || false);
    setHasPrivacyPolicy(status?.privacy_policy_enabled || false);
  }, [status]);

  useEffect(() => {
    isPasskeySupported()
      .then(setPasskeySupported)
      .catch(() => setPasskeySupported(false));

    return () => {
      if (githubTimeoutRef.current) {
        clearTimeout(githubTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (searchParams.get('expired')) {
      showError(t('未登录或登录已过期，请重新登录'));
    }
  }, [searchParams, t]);

  const completeLogin = (data, targetPath) => {
    userDispatch({ type: 'login', payload: data });
    setUserData(data);
    updateAPI();
    showSuccess('登录成功！');
    navigate(targetPath);
  };

  const ensureAgreementAccepted = () => {
    if (requiresAgreement && !agreedToTerms) {
      showInfo(t('请先阅读并同意用户协议和隐私政策'));
      return false;
    }
    return true;
  };

  const handleChange = (name, value) => {
    setInputs((currentInputs) => ({ ...currentInputs, [name]: value }));
  };

  const handleSubmit = async (event) => {
    event?.preventDefault?.();
    if (!ensureAgreementAccepted()) {
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
      return;
    }

    setLoginLoading(true);
    try {
      if (!username || !password) {
        showError('请输入用户名和密码！');
        return;
      }

      const res = await API.post(`/api/user/login?turnstile=${turnstileToken}`, {
        username,
        password,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }

      if (data && data.require_2fa) {
        setShowTwoFA(true);
        return;
      }

      completeLogin(data, '/console');
      if (username === 'root' && password === '123456') {
        Modal.error({
          title: '您正在使用默认密码！',
          content: '请立刻修改默认密码！',
          centered: true,
        });
      }
    } catch (error) {
      showError('登录失败，请重试');
    } finally {
      setLoginLoading(false);
    }
  };

  const onWeChatLoginClicked = () => {
    if (!ensureAgreementAccepted()) {
      return;
    }
    setWechatLoading(true);
    setShowWeChatLoginModal(true);
    setWechatLoading(false);
  };

  const onSubmitWeChatVerificationCode = async () => {
    if (turnstileEnabled && turnstileToken === '') {
      showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
      return;
    }
    setWechatCodeSubmitLoading(true);
    try {
      const res = await API.get(
        `/api/oauth/wechat?code=${inputs.wechat_verification_code}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        setUserData(data);
        updateAPI();
        navigate('/');
        showSuccess('登录成功！');
        setShowWeChatLoginModal(false);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('登录失败，请重试');
    } finally {
      setWechatCodeSubmitLoading(false);
    }
  };

  const onTelegramLoginClicked = async (response) => {
    if (!ensureAgreementAccepted()) {
      return;
    }

    const fields = [
      'id',
      'first_name',
      'last_name',
      'username',
      'photo_url',
      'auth_date',
      'hash',
      'lang',
    ];
    const params = {};
    fields.forEach((field) => {
      if (response[field]) {
        params[field] = response[field];
      }
    });

    try {
      const res = await API.get('/api/oauth/telegram/login', { params });
      const { success, message, data } = res.data;
      if (success) {
        completeLogin(data, '/');
      } else {
        showError(message);
      }
    } catch (error) {
      showError('登录失败，请重试');
    }
  };

  const handleGitHubClick = () => {
    if (!ensureAgreementAccepted() || githubButtonDisabled) {
      return;
    }
    setGithubLoading(true);
    setGithubButtonDisabled(true);
    setGithubButtonState('redirecting');
    if (githubTimeoutRef.current) {
      clearTimeout(githubTimeoutRef.current);
    }
    githubTimeoutRef.current = setTimeout(() => {
      setGithubLoading(false);
      setGithubButtonState('timeout');
      setGithubButtonDisabled(true);
    }, 20000);
    try {
      onGitHubOAuthClicked(status.github_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setGithubLoading(false), 3000);
    }
  };

  const handleDiscordClick = () => {
    if (!ensureAgreementAccepted()) {
      return;
    }
    setDiscordLoading(true);
    try {
      onDiscordOAuthClicked(status.discord_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setDiscordLoading(false), 3000);
    }
  };

  const handleOIDCClick = () => {
    if (!ensureAgreementAccepted()) {
      return;
    }
    setOidcLoading(true);
    try {
      onOIDCClicked(
        status.oidc_authorization_endpoint,
        status.oidc_client_id,
        false,
        { shouldLogout: true },
      );
    } finally {
      setTimeout(() => setOidcLoading(false), 3000);
    }
  };

  const handleLinuxDOClick = () => {
    if (!ensureAgreementAccepted()) {
      return;
    }
    setLinuxdoLoading(true);
    try {
      onLinuxDOOAuthClicked(status.linuxdo_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setLinuxdoLoading(false), 3000);
    }
  };

  const handleCustomOAuthClick = (provider) => {
    if (!ensureAgreementAccepted()) {
      return;
    }
    setCustomOAuthLoading((prev) => ({ ...prev, [provider.slug]: true }));
    try {
      onCustomOAuthClicked(provider, { shouldLogout: true });
    } finally {
      setTimeout(() => {
        setCustomOAuthLoading((prev) => ({ ...prev, [provider.slug]: false }));
      }, 3000);
    }
  };

  const handlePasskeyLogin = async () => {
    if (!ensureAgreementAccepted()) {
      return;
    }
    if (!passkeySupported) {
      showInfo('当前环境无法使用 Passkey 登录');
      return;
    }
    if (!window.PublicKeyCredential) {
      showInfo('当前浏览器不支持 Passkey');
      return;
    }

    setPasskeyLoading(true);
    try {
      const beginRes = await API.post('/api/user/passkey/login/begin');
      const { success, message, data } = beginRes.data;
      if (!success) {
        showError(message || '无法发起 Passkey 登录');
        return;
      }

      const publicKeyOptions = prepareCredentialRequestOptions(
        data?.options || data?.publicKey || data,
      );
      const assertion = await navigator.credentials.get({
        publicKey: publicKeyOptions,
      });
      const payload = buildAssertionResult(assertion);
      if (!payload) {
        showError('Passkey 验证失败，请重试');
        return;
      }

      const finishRes = await API.post('/api/user/passkey/login/finish', payload);
      const finish = finishRes.data;
      if (finish.success) {
        completeLogin(finish.data, '/console');
      } else {
        showError(finish.message || 'Passkey 登录失败，请重试');
      }
    } catch (error) {
      if (error?.name === 'AbortError') {
        showInfo('已取消 Passkey 登录');
      } else {
        showError('Passkey 登录失败，请重试');
      }
    } finally {
      setPasskeyLoading(false);
    }
  };

  const handleResetPasswordClick = () => {
    setResetPasswordLoading(true);
    navigate('/reset');
    setResetPasswordLoading(false);
  };

  const handle2FASuccess = (data) => {
    completeLogin(data, '/console');
  };

  const handleBackToLogin = () => {
    setShowTwoFA(false);
    setInputs({ username: '', password: '', wechat_verification_code: '' });
  };

  const renderAgreement = () => {
    if (!requiresAgreement) {
      return null;
    }

    return (
      <div className='landing-login__agreement'>
        <Checkbox
          checked={agreedToTerms}
          onChange={(event) => setAgreedToTerms(event.target.checked)}
        >
          <span className='landing-login__agreementText'>
            {t('我已阅读并同意')}
            {hasUserAgreement && (
              <a
                href='/user-agreement'
                target='_blank'
                rel='noopener noreferrer'
                className='landing-login__agreementLink'
              >
                {t('用户协议')}
              </a>
            )}
            {hasUserAgreement && hasPrivacyPolicy ? t('和') : ''}
            {hasPrivacyPolicy && (
              <a
                href='/privacy-policy'
                target='_blank'
                rel='noopener noreferrer'
                className='landing-login__agreementLink'
              >
                {t('隐私政策')}
              </a>
            )}
          </span>
        </Checkbox>
      </div>
    );
  };

  const renderAlternativeLoginButtons = () => {
    if (!hasAlternativeLoginOptions) {
      return null;
    }

    return (
      <div className='landing-login__oauthList'>
        {hasPasskeyLogin && (
          <Button
            theme='outline'
            type='tertiary'
            className='landing-login__oauthButton'
            icon={<IconKey size='large' />}
            onClick={handlePasskeyLogin}
            loading={passkeyLoading}
          >
            <span>{t('使用 Passkey 登录')}</span>
          </Button>
        )}

        {status.wechat_login && (
          <Button
            theme='outline'
            type='tertiary'
            className='landing-login__oauthButton'
            icon={<Icon svg={<WeChatIcon />} style={{ color: '#07C160' }} />}
            onClick={onWeChatLoginClicked}
            loading={wechatLoading}
          >
            <span>{t('使用 微信 继续')}</span>
          </Button>
        )}

        {status.github_oauth && (
          <Button
            theme='outline'
            type='tertiary'
            className='landing-login__oauthButton'
            icon={<IconGithubLogo size='large' />}
            onClick={handleGitHubClick}
            loading={githubLoading}
            disabled={githubButtonDisabled}
          >
            <span>{githubButtonText}</span>
          </Button>
        )}

        {status.discord_oauth && (
          <Button
            theme='outline'
            type='tertiary'
            className='landing-login__oauthButton'
            icon={
              <SiDiscord
                style={{
                  color: '#5865F2',
                  width: '18px',
                  height: '18px',
                }}
              />
            }
            onClick={handleDiscordClick}
            loading={discordLoading}
          >
            <span>{t('使用 Discord 继续')}</span>
          </Button>
        )}

        {status.oidc_enabled && (
          <Button
            theme='outline'
            type='tertiary'
            className='landing-login__oauthButton'
            icon={<OIDCIcon style={{ color: '#5cb5ff' }} />}
            onClick={handleOIDCClick}
            loading={oidcLoading}
          >
            <span>{t('使用 OIDC 继续')}</span>
          </Button>
        )}

        {status.linuxdo_oauth && (
          <Button
            theme='outline'
            type='tertiary'
            className='landing-login__oauthButton'
            icon={
              <LinuxDoIcon
                style={{
                  color: '#E95420',
                  width: '18px',
                  height: '18px',
                }}
              />
            }
            onClick={handleLinuxDOClick}
            loading={linuxdoLoading}
          >
            <span>{t('使用 LinuxDO 继续')}</span>
          </Button>
        )}

        {status.custom_oauth_providers &&
          status.custom_oauth_providers.map((provider) => (
            <Button
              key={provider.slug}
              theme='outline'
              type='tertiary'
              className='landing-login__oauthButton'
              icon={getOAuthProviderIcon(provider.icon || '', 18)}
              onClick={() => handleCustomOAuthClick(provider)}
              loading={customOAuthLoading[provider.slug]}
            >
              <span>{t('使用 {{name}} 继续', { name: provider.name })}</span>
            </Button>
          ))}

        {status.telegram_oauth && (
          <div className='landing-login__telegramWrap'>
            <TelegramLoginButton
              dataOnauth={onTelegramLoginClicked}
              botName={status.telegram_bot_name}
            />
          </div>
        )}
      </div>
    );
  };

  const renderWeChatLoginModal = () => {
    return (
      <Modal
        title={t('微信扫码登录')}
        visible={showWeChatLoginModal}
        maskClosable={true}
        onOk={onSubmitWeChatVerificationCode}
        onCancel={() => setShowWeChatLoginModal(false)}
        okText={t('登录')}
        centered={true}
        okButtonProps={{
          loading: wechatCodeSubmitLoading,
        }}
      >
        <div className='landing-login__wechatModal'>
          <img
            src={status.wechat_qrcode}
            alt='微信二维码'
            className='landing-login__wechatQr'
          />
          <p className='landing-login__wechatHint'>
            {t('微信扫码关注公众号，输入「验证码」获取验证码（三分钟内有效）')}
          </p>
          <Input
            placeholder={t('验证码')}
            value={inputs.wechat_verification_code}
            onChange={(value) => handleChange('wechat_verification_code', value)}
          />
        </div>
      </Modal>
    );
  };

  const render2FAModal = () => {
    return (
      <Modal
        title={t('两步验证')}
        visible={showTwoFA}
        onCancel={handleBackToLogin}
        footer={null}
        width={450}
        centered
      >
        <TwoFAVerification
          onSuccess={handle2FASuccess}
          onBack={handleBackToLogin}
          isModal={true}
        />
      </Modal>
    );
  };

  return (
    <MarketingShell activeNav='login' className='landing-login'>
      <section className='landing-login__section'>
        <div className='landing-login__grid'>
          <div className='landing-login__content'>
            <div className='landing-login__eyebrow'>
              <Sparkles size={14} />
              <span>{t('统一 AI 访问入口')}</span>
            </div>

            <h1 className='landing-login__title'>
              {t('登录后，即刻启用')}
              <br />
              <span className='landing-login__titleAccent'>{systemName}</span>
              {t(' 的完整能力')}
            </h1>

            <p className='landing-login__description'>
              {t(
                '进入控制台即可查看模型价格、创建 API 令牌、管理调用与账户配置，所有主流模型能力统一接入。',
              )}
            </p>

            <div className='landing-login__actions'>
              <Link to='/pricing' className='landing-home__primaryButton landing-login__primaryLink'>
                {t('查看模型价格')}
                <ArrowRight size={16} />
              </Link>
              <div className='landing-login__signal'>
                <span>{t('多方式登录')}</span>
                <span>{t('控制台直达')}</span>
                <span>{t('统一计费')}</span>
              </div>
            </div>

            <div className='landing-login__featureGrid'>
              {featureCards.map((feature) => {
                const FeatureIcon = feature.icon;
                return (
                  <article key={feature.title} className='landing-login__featureCard'>
                    <span className='landing-login__featureIcon'>
                      <FeatureIcon size={18} />
                    </span>
                    <div>
                      <h3>{feature.title}</h3>
                      <p>{feature.description}</p>
                    </div>
                  </article>
                );
              })}
            </div>

            <div className='landing-login__trustPanel'>
              <div className='landing-login__trustMetric'>
                <strong>45+</strong>
                <span>{t('主流模型统一调度')}</span>
              </div>
              <div className='landing-login__trustMetric'>
                <strong>2FA</strong>
                <span>{t('支持二次验证')}</span>
              </div>
              <div className='landing-login__trustMetric'>
                <strong>Passkey</strong>
                <span>{t('免密登录可选')}</span>
              </div>
            </div>
          </div>

          <div className='landing-login__panelWrap'>
            <div className='landing-login__panelGlow' />
            <div className='landing-login__card'>
              <div className='landing-login__cardHeader'>
                <div>
                  <p className='landing-login__cardEyebrow'>{t('欢迎回来')}</p>
                  <h2>{t('登录')}</h2>
                </div>
                <span className='landing-login__cardBadge'>
                  <Fingerprint size={14} />
                  {t('安全认证')}
                </span>
              </div>

              <p className='landing-login__cardDescription'>
                {t('使用你的账户继续访问控制台与模型能力。')}
              </p>

              <form className='landing-login__form' onSubmit={handleSubmit}>
                <div className='landing-login__field'>
                  <label htmlFor='login-username'>{t('用户名或邮箱')}</label>
                  <Input
                    id='login-username'
                    className='landing-login__input'
                    size='large'
                    prefix={<IconMail />}
                    placeholder={t('请输入您的用户名或邮箱地址')}
                    value={inputs.username}
                    onChange={(value) => handleChange('username', value)}
                  />
                </div>

                <div className='landing-login__field'>
                  <label htmlFor='login-password'>{t('密码')}</label>
                  <Input
                    id='login-password'
                    className='landing-login__input'
                    size='large'
                    prefix={<IconLock />}
                    placeholder={t('请输入您的密码')}
                    mode='password'
                    value={inputs.password}
                    onChange={(value) => handleChange('password', value)}
                  />
                </div>

                {renderAgreement()}

                <Button
                  theme='solid'
                  type='primary'
                  htmlType='submit'
                  className='landing-login__submitButton'
                  loading={loginLoading}
                  disabled={requiresAgreement && !agreedToTerms}
                >
                  {t('继续')}
                </Button>
              </form>

              <div className='landing-login__helperRow'>
                <Button
                  theme='borderless'
                  type='tertiary'
                  className='landing-login__helperButton'
                  onClick={handleResetPasswordClick}
                  loading={resetPasswordLoading}
                >
                  {t('忘记密码？')}
                </Button>

                {!status.self_use_mode_enabled && (
                  <p className='landing-login__registerHint'>
                    {t('没有账户？')}
                    <Link to='/register'>{t('注册')}</Link>
                  </p>
                )}
              </div>

              {hasAlternativeLoginOptions && (
                <>
                  <Divider margin='18px' align='center'>
                    {t('或使用其他方式')}
                  </Divider>
                  {renderAlternativeLoginButtons()}
                </>
              )}

              {turnstileEnabled && (
                <div className='landing-login__turnstileWrap'>
                  <span className='landing-login__turnstileLabel'>
                    {t('安全校验')}
                  </span>
                  <Turnstile
                    sitekey={turnstileSiteKey}
                    onVerify={(token) => {
                      setTurnstileToken(token);
                    }}
                  />
                </div>
              )}
            </div>
          </div>
        </div>
      </section>

      {renderWeChatLoginModal()}
      {render2FAModal()}
    </MarketingShell>
  );
};

export default LoginForm;
