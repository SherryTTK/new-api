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

import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import {
  ArrowRight,
  BadgeCheck,
  ChevronRight,
  Layers3,
  ShieldCheck,
  SlidersHorizontal,
} from 'lucide-react';
import { getSystemName } from '../../helpers';
import MarketingShell from '../../components/marketing/MarketingShell';

const providerTokens = [
  { label: 'OA', className: 'landing-home__provider--blue' },
  { label: 'CL', className: 'landing-home__provider--orange' },
  { label: 'GM', className: 'landing-home__provider--violet' },
  { label: 'DS', className: 'landing-home__provider--cyan' },
  { label: 'QW', className: 'landing-home__provider--white' },
];

const Landing = () => {
  const { t } = useTranslation();
  const [openFaqIndex, setOpenFaqIndex] = useState(null);
  const systemName = getSystemName();

  const stats = [
    {
      value: '99.9%',
      title: t('高可用托管'),
      description: t('全平台智能就近运行，多节点负载均衡'),
    },
    {
      value: 'OpenAI',
      title: t('主流生态兼容'),
      description: t('全格式兼容，无缝对接已有程序'),
    },
    {
      value: t('毫秒级'),
      title: t('极速响应'),
      description: t('链路聚合调度加持，低延迟高并发'),
    },
  ];

  const features = [
    {
      icon: SlidersHorizontal,
      title: t('接入超简单'),
      description: t(
        '兼容主流 SDK、应用及工作流，无需学习额外文档，三分钟就能跑通。',
      ),
      tags: [],
    },
    {
      icon: Layers3,
      title: t('全模型覆盖'),
      description: t(
        '主流语言模型、图像、代码及多模态能力统一接入，业务方案从此集中管理。',
      ),
      tags: [t('Text'), t('Chat'), t('Embedding'), t('Images')],
    },
    {
      icon: ShieldCheck,
      title: t('极速且稳定'),
      description: t(
        '企业级线路优化、毫秒级调度，万人同时调用也不掉线。',
      ),
      tags: [],
    },
    {
      icon: BadgeCheck,
      title: t('高性价比之选'),
      description: t(
        '按量计费，可选按日或按次结算费用，月多一分多付，成本可控。',
      ),
      tags: [],
    },
  ];

  const steps = [
    {
      index: '01',
      title: t('注册登录账户'),
      description: t(
        '使用邮箱快速注册，无需复杂验证，10 秒即可完成账户创建。',
      ),
    },
    {
      index: '02',
      title: t('控制台添加令牌'),
      description: t(
        '前往控制台 API 页面获取身份令牌，支持多令牌管理，灵活控制访问权限。',
      ),
    },
    {
      index: '03',
      title: t('第三方工具配置'),
      description: t(
        '在 API 或现有应用里填写 API 地址和令牌密钥，标准配置到位即可开始调用 AI 模型接口。',
      ),
    },
  ];

  const faqs = [
    {
      question: `${systemName}${t(' 平台是什么？')}`,
      answer: t(
        '这是一个聚合多家大模型服务的统一入口，你可以使用同一套调用方式管理不同模型供应商。',
      ),
    },
    {
      question: t('定价如何运作？'),
      answer: t(
        '当前页面仅完成静态展示，后续可以按你的业务模式接入按量计费、套餐或会员体系。',
      ),
    },
    {
      question: t('有隐藏费用吗？'),
      answer: t(
        '页面文案可完全按你的品牌和业务规则调整，目前只是还原落地页的设计效果，不绑定真实计费逻辑。',
      ),
    },
    {
      question: t('支持哪些端点？'),
      answer: t(
        '可以继续接入聊天、补全、图像、向量、音频等接口，静态页面结构已经预留了扩展空间。',
      ),
    },
  ];

  return (
    <MarketingShell activeNav='home'>
      <section className='landing-home__hero'>
        <div className='landing-home__heroDots' />
        <div className='landing-home__heroContent'>
          <p className='landing-home__eyebrow'>{t('全球智能网关')}</p>
          <h1 className='landing-home__heroTitle'>
            {t('汇聚全球智慧')}
            <br />
            <span className='landing-home__heroTitleAccent'>{systemName}</span>
            {t('为您一键调度')}
          </h1>

          <div className='landing-home__heroBadge'>
            <span>{t('稳定')}</span>
            <span>{t('便宜')}</span>
            <span>{t('快')}</span>
          </div>

          <p className='landing-home__heroDescription'>
            {t(
              '接入 AI 就像换个 API 地址那么简单，前端无感对接 OpenAI、Gemini、Claude、Suno、文生图等全球主流模型。',
            )}
          </p>

          <div className='landing-home__heroActions'>
            <Link to='/console' className='landing-home__primaryButton'>
              {t('立即开始')}
              <ArrowRight size={16} />
            </Link>

            <div className='landing-home__providers'>
              <div className='landing-home__providerList'>
                {providerTokens.map((provider) => (
                  <span
                    key={provider.label}
                    className={`landing-home__provider ${provider.className}`}
                  >
                    {provider.label}
                  </span>
                ))}
              </div>
              <span className='landing-home__providersText'>45+</span>
            </div>
          </div>
        </div>

        <div className='landing-home__arc' />
      </section>

      <section className='landing-home__statsSection'>
        <div className='landing-home__statsGrid'>
          {stats.map((stat) => (
            <article key={stat.value} className='landing-home__statCard'>
              <div className='landing-home__statValue'>{stat.value}</div>
              <h2 className='landing-home__statTitle'>{stat.title}</h2>
              <p className='landing-home__statDescription'>{stat.description}</p>
            </article>
          ))}
        </div>
      </section>

      <section className='landing-home__section'>
        <div className='landing-home__sectionHeading'>
          <h2>
            {t('把复杂交给')} {systemName}
          </h2>
          <p>{t('拒绝大海捞针，目标只有一个，让你更快用上最好用的 AI。')}</p>
        </div>

        <div className='landing-home__featureGrid'>
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <article key={feature.title} className='landing-home__featureCard'>
                <div className='landing-home__featureHeader'>
                  <span className='landing-home__featureIcon'>
                    <Icon size={18} />
                  </span>
                  <h3>{feature.title}</h3>
                </div>
                <p>{feature.description}</p>
                {feature.tags.length > 0 && (
                  <div className='landing-home__tagRow'>
                    {feature.tags.map((tag) => (
                      <span key={tag} className='landing-home__tag'>
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </article>
            );
          })}
        </div>
      </section>

      <section className='landing-home__section landing-home__section--steps'>
        <div className='landing-home__sectionHeading'>
          <h2>{t('三步接入，即刻启程')}</h2>
          <p>{t('10 秒注册，一键生成令牌，填入软件或代码，即刻调用。')}</p>
        </div>

        <div className='landing-home__steps'>
          {steps.map((step, index) => (
            <article key={step.index} className='landing-home__stepItem'>
              <div className='landing-home__stepIndex'>
                <span>{step.index}</span>
                {index < steps.length - 1 && (
                  <div className='landing-home__stepLine' />
                )}
              </div>

              <div className='landing-home__stepCard'>
                <span className='landing-home__stepDot' />
                <div>
                  <h3>{step.title}</h3>
                  <p>{step.description}</p>
                </div>
              </div>
            </article>
          ))}
        </div>
      </section>

      <section className='landing-home__section landing-home__section--faq' id='faq'>
        <div className='landing-home__sectionHeading'>
          <h2>{t('常见问题')}</h2>
          <p>{t('用最直接的方式回答你关心的细节。')}</p>
        </div>

        <div className='landing-home__faqList'>
          {faqs.map((faq, index) => {
            const open = openFaqIndex === index;
            return (
              <article key={faq.question} className='landing-home__faqItem'>
                <button
                  type='button'
                  className='landing-home__faqButton'
                  onClick={() => setOpenFaqIndex(open ? null : index)}
                >
                  <span className='landing-home__faqLeading' />
                  <span className='landing-home__faqQuestion'>
                    {faq.question}
                  </span>
                  <ChevronRight
                    size={18}
                    className={`landing-home__faqChevron${open ? ' landing-home__faqChevron--open' : ''}`}
                  />
                </button>

                {open && <p className='landing-home__faqAnswer'>{faq.answer}</p>}
              </article>
            );
          })}
        </div>
      </section>
    </MarketingShell>
  );
};

export default Landing;
