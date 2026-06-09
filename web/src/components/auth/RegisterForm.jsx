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
import { Link, useNavigate } from 'react-router-dom';
import { Button, Checkbox, Divider, Icon, Input, Modal } from '@douyinfe/semi-ui';
import {
  IconGithubLogo,
  IconKey,
  IconLock,
  IconMail,
  IconUser,
} from '@douyinfe/semi-icons';
import { ArrowRight, BadgeCheck, Layers3, ShieldCheck, Sparkles } from 'lucide-react';
import { SiDiscord } from 'react-icons/si';
import Turnstile from 'react-turnstile';
import TelegramLoginButton from 'react-telegram-login';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import {
  API,
  getOAuthProviderIcon,
  getSystemName,
  onCustomOAuthClicked,
  onDiscordOAuthClicked,
  onGitHubOAuthClicked,
  onLinuxDOOAuthClicked,
  onOIDCClicked,
  setUserData,
  showError,
  showInfo,
  showSuccess,
  updateAPI,
} from '../../helpers';
import OIDCIcon from '../common/logo/OIDCIcon';
import LinuxDoIcon from '../common/logo/LinuxDoIcon';
import WeChatIcon from '../common/logo/WeChatIcon';
import MarketingShell from '../marketing/MarketingShell';

const RegisterForm = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();
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
    password2: '',
    email: '',
    verification_code: '',
    wechat_verification_code: '',
  });
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);
  const [wechatLoading, setWechatLoading] = useState(false);
  const [githubLoading, setGithubLoading] = useState(false);
  const [discordLoading, setDiscordLoading] = useState(false);
  const [oidcLoading, setOidcLoading] = useState(false);
  const [linuxdoLoading, setLinuxdoLoading] = useState(false);
  const [registerLoading, setRegisterLoading] = useState(false);
  const [verificationCodeLoading, setVerificationCodeLoading] = useState(false);
  const [wechatCodeSubmitLoading, setWechatCodeSubmitLoading] = useState(false);
  const [customOAuthLoading, setCustomOAuthLoading] = useState({});
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [hasUserAgreement, setHasUserAgreement] = useState(false);
  const [hasPrivacyPolicy, setHasPrivacyPolicy] = useState(false);
  const [githubButtonState, setGithubButtonState] = useState('idle');
  const [githubButtonDisabled, setGithubButtonDisabled] = useState(false);
  const [showEmailVerification, setShowEmailVerification] = useState(false);
  const githubTimeoutRef = useRef(null);
  const githubButtonText = t(githubButtonTextKeyByState[githubButtonState]);
  const systemName = getSystemName();
  const requiresAgreement = hasUserAgreement || hasPrivacyPolicy;

  const featureCards = [
    {
      icon: ShieldCheck,
      title: t('稳定上线'),
      description: t('统一接入全球主流模型，创建账户后即可开始调用。'),
    },
    {
      icon: Layers3,
      title: t('生态兼容'),
      description: t('兼容 OpenAI 生态与常见工具，无需大改现有代码。'),
    },
    {
      icon: BadgeCheck,
      title: t('毫秒级体验'),
      description: t('完成注册后可立即进入控制台创建密钥并开始接入。'),
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
  const hasOAuthRegisterOptions = Boolean(
    status.github_oauth ||
      status.discord_oauth ||
      status.oidc_enabled ||
      status.wechat_login ||
      status.linuxdo_oauth ||
      status.telegram_oauth ||
      hasCustomOAuthProviders,
  );

  let affCode = new URLSearchParams(window.location.search).get('aff');
  if (affCode) {
    localStorage.setItem('aff', affCode);
  }

  useEffect(() => {
    setShowEmailVerification(!!status?.email_verification);
    if (status?.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(status.turnstile_site_key);
    }
    setHasUserAgreement(status?.user_agreement_enabled || false);
    setHasPrivacyPolicy(status?.privacy_policy_enabled || false);
  }, [status]);

  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown((current) => current - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(countdownInterval);
  }, [disableButton, countdown]);

  useEffect(() => {
    return () => {
      if (githubTimeoutRef.current) {
        clearTimeout(githubTimeoutRef.current);
      }
    };
  }, []);

  const ensureAgreementAccepted = () => {
    if (requiresAgreement && !agreedToTerms) {
      showInfo(t('请先阅读并同意用户协议和隐私政策'));
      return false;
    }
    return true;
  };

  const completeOAuthLogin = (data) => {
    userDispatch({ type: 'login', payload: data });
    setUserData(data);
    updateAPI();
    showSuccess('登录成功！');
    navigate('/');
  };

  const handleChange = (name, value) => {
    setInputs((currentInputs) => ({ ...currentInputs, [name]: value }));
  };

  const handleSubmit = async (event) => {
    event?.preventDefault?.();
    if (!ensureAgreementAccepted()) {
      return;
    }
    if (inputs.password.length < 8) {
      showInfo('密码长度不得小于 8 位！');
      return;
    }
    if (inputs.password !== inputs.password2) {
      showInfo('两次输入的密码不一致');
      return;
    }
    if (!inputs.username || !inputs.password) {
      showInfo('请输入用户名和密码！');
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
      return;
    }

    setRegisterLoading(true);
    try {
      if (!affCode) {
        affCode = localStorage.getItem('aff');
      }
      const payload = {
        ...inputs,
        aff_code: affCode,
      };
      const res = await API.post(
        `/api/user/register?turnstile=${turnstileToken}`,
        payload,
      );
      const { success, message } = res.data;
      if (success) {
        navigate('/login');
        showSuccess('注册成功！');
      } else {
        showError(message);
      }
    } catch (error) {
      showError('注册失败，请重试');
    } finally {
      setRegisterLoading(false);
    }
  };

  const sendVerificationCode = async () => {
    if (inputs.email === '') {
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo('请稍后几秒重试，Turnstile 正在检查用户环境！');
      return;
    }
    setVerificationCodeLoading(true);
    try {
      const res = await API.get(
        `/api/verification?email=${encodeURIComponent(inputs.email)}&turnstile=${turnstileToken}`,
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess('验证码发送成功，请检查你的邮箱！');
        setDisableButton(true);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('发送验证码失败，请重试');
    } finally {
      setVerificationCodeLoading(false);
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
        completeOAuthLogin(data);
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
        completeOAuthLogin(data);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('登录失败，请重试');
    }
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

  const renderAlternativeRegisterButtons = () => {
    if (!hasOAuthRegisterOptions) {
      return null;
    }

    return (
      <div className='landing-login__oauthList'>
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

  return (
    <MarketingShell activeNav='register' className='landing-login landing-register'>
      <section className='landing-login__section'>
        <div className='landing-login__grid'>
          <div className='landing-login__content'>
            <div className='landing-login__eyebrow'>
              <Sparkles size={14} />
              <span>{t('稳定 · 便宜 · 快')}</span>
            </div>

            <h1 className='landing-login__title'>
              {t('创建你的')}
              <br />
              <span className='landing-login__titleAccent'>{t('专属密钥')}</span>
            </h1>

            <p className='landing-login__description'>
              {t(
                '10 秒完成注册，统一接入全球主流模型，马上开启你的 AI 之旅。',
              )}
            </p>

            <div className='landing-login__actions'>
              <Link to='/pricing' className='landing-home__primaryButton landing-login__primaryLink'>
                {t('浏览模型与价格')}
                <ArrowRight size={16} />
              </Link>
              <div className='landing-login__signal'>
                <span>{t('注册即用')}</span>
                <span>{t('统一密钥')}</span>
                <span>{t('多模型接入')}</span>
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
                <strong>99.9%</strong>
                <span>{t('高可用保障')}</span>
              </div>
              <div className='landing-login__trustMetric'>
                <strong>OpenAI</strong>
                <span>{t('全格式兼容')}</span>
              </div>
              <div className='landing-login__trustMetric'>
                <strong>{t('毫秒级')}</strong>
                <span>{t('低延迟访问')}</span>
              </div>
            </div>
          </div>

          <div className='landing-login__panelWrap'>
            <div className='landing-login__panelGlow' />
            <div className='landing-login__card'>
              <div className='landing-login__cardHeader'>
                <div>
                  <p className='landing-login__cardEyebrow'>SIGN UP</p>
                  <h2>{t('创建新账户')}</h2>
                </div>
              </div>

              <form className='landing-login__form' onSubmit={handleSubmit}>
                <div className='landing-login__field'>
                  <label htmlFor='register-username'>{t('用户名或邮箱')}</label>
                  <Input
                    id='register-username'
                    className='landing-login__input'
                    size='large'
                    prefix={<IconUser />}
                    autoComplete='username'
                    placeholder={t('请输入用户名')}
                    value={inputs.username}
                    onChange={(value) => handleChange('username', value)}
                  />
                </div>

                <div className='landing-login__field'>
                  <label htmlFor='register-password'>{t('密码')}</label>
                  <Input
                    id='register-password'
                    className='landing-login__input'
                    size='large'
                    prefix={<IconLock />}
                    autoComplete='new-password'
                    placeholder={t('输入密码，最短 8 位，最长 20 位')}
                    mode='password'
                    value={inputs.password}
                    onChange={(value) => handleChange('password', value)}
                  />
                </div>

                <div className='landing-login__field'>
                  <label htmlFor='register-password2'>{t('确认密码')}</label>
                  <Input
                    id='register-password2'
                    className='landing-login__input'
                    size='large'
                    prefix={<IconLock />}
                    autoComplete='new-password'
                    placeholder={t('确认密码')}
                    mode='password'
                    value={inputs.password2}
                    onChange={(value) => handleChange('password2', value)}
                  />
                </div>

                {showEmailVerification && (
                  <>
                    <div className='landing-login__field'>
                      <label htmlFor='register-email'>{t('邮箱')}</label>
                      <Input
                        id='register-email'
                        className='landing-login__input'
                        size='large'
                        prefix={<IconMail />}
                        autoComplete='email'
                        placeholder={t('输入邮箱地址')}
                        value={inputs.email}
                        onChange={(value) => handleChange('email', value)}
                        suffix={
                          <Button
                            htmlType='button'
                            theme='borderless'
                            type='tertiary'
                            className='landing-register__verifyButton'
                            onClick={sendVerificationCode}
                            loading={verificationCodeLoading}
                            disabled={disableButton || verificationCodeLoading}
                          >
                            {disableButton
                              ? `${t('重新发送')} (${countdown})`
                              : t('获取验证码')}
                          </Button>
                        }
                      />
                    </div>

                    <div className='landing-login__field'>
                      <label htmlFor='register-verification'>
                        {t('验证码')}
                      </label>
                      <Input
                        id='register-verification'
                        className='landing-login__input'
                        size='large'
                        prefix={<IconKey />}
                        placeholder={t('输入验证码')}
                        value={inputs.verification_code}
                        onChange={(value) =>
                          handleChange('verification_code', value)
                        }
                      />
                    </div>
                  </>
                )}

                {renderAgreement()}

                <Button
                  theme='solid'
                  type='primary'
                  htmlType='submit'
                  className='landing-login__submitButton'
                  loading={registerLoading}
                  disabled={requiresAgreement && !agreedToTerms}
                >
                  {t('继续')}
                </Button>
              </form>

              <div className='landing-login__helperRow'>
                <div className='landing-register__helperNote'>
                  {showEmailVerification
                    ? t('邮箱验证开启后，需先获取验证码再完成注册。')
                    : t('创建成功后即可前往登录页开始使用。')}
                </div>

                <p className='landing-login__registerHint'>
                  {t('已有账户？')}
                  <Link to='/login'>{t('登录')}</Link>
                </p>
              </div>

              {hasOAuthRegisterOptions && (
                <>
                  <Divider margin='18px' align='center'>
                    {t('或使用其他方式')}
                  </Divider>
                  {renderAlternativeRegisterButtons()}
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
    </MarketingShell>
  );
};

export default RegisterForm;
