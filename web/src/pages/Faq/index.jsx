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

import React, { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import MarketingShell from '../../components/marketing/MarketingShell';
import { getSystemName } from '../../helpers';

const categoryDefs = [
  { id: 'all', icon: '📚', label: '所有问题' },
  { id: 'general', icon: '🔍', label: '一般问题' },
  { id: 'pricing', icon: '💰', label: '定价与计费' },
  { id: 'api', icon: '⚡', label: 'API使用' },
  { id: 'account', icon: '🔒', label: '账户与安全' },
  { id: 'technical', icon: '🛠️', label: '技术支持' },
  { id: 'integration', icon: '🔗', label: '集成' },
];

const Faq = () => {
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [openQuestionId, setOpenQuestionId] = useState(null);
  const [showBackTop, setShowBackTop] = useState(false);
  const systemName = getSystemName();

  React.useEffect(() => {
    const onScroll = () => {
      setShowBackTop(window.scrollY > 240);
    };
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  const categories = useMemo(
    () =>
      categoryDefs.map((item) => ({
        ...item,
        name: t(item.label),
      })),
    [t],
  );

  const questions = useMemo(
    () => [
      {
        id: 1,
        category: 'general',
        question: t('什么是API平台？'),
        answer: t(
          `${systemName} 是一个统一的大模型 API 接入平台。你可以通过同一套鉴权与调用方式，集中接入不同厂商的文本、图像和多模态模型，减少重复适配工作。`,
        ),
      },
      {
        id: 2,
        category: 'general',
        question: t('如何开始使用？'),
        answer: t(
          '注册并登录后，先在控制台创建 API 令牌，再将 Base URL 与令牌配置到你的 SDK、应用或工作流工具里，就可以开始调用模型了。',
        ),
      },
      {
        id: 3,
        category: 'general',
        question: t('有哪些可用的模型？'),
        answer: t(
          '平台通常覆盖主流对话模型、图像生成模型以及部分音频、向量和视频能力。你可以前往模型页查看当前已开放的具体模型与价格信息。',
        ),
      },
      {
        id: 4,
        category: 'pricing',
        question: t('定价如何运作？'),
        answer: t(
          '不同模型会按各自的输入、输出或按次规则计费。页面展示的价格会随实际配置更新，建议在下单或正式接入前，以模型页中的实时价格为准。',
        ),
      },
      {
        id: 5,
        category: 'pricing',
        question: t('您接受哪些付款方式？'),
        answer: t(
          '可接入的支付方式取决于当前站点运营配置，常见方式包括余额充值、在线支付或人工结算。如需确认，请以充值页展示为准。',
        ),
      },
      {
        id: 6,
        category: 'pricing',
        question: t('我可以退款吗？'),
        answer: t(
          '是否支持退款取决于账户状态、使用情况与站点规则。若额度尚未消耗且符合运营策略，可以联系平台支持团队进一步核实。',
        ),
      },
      {
        id: 7,
        category: 'pricing',
        question: t('有隐藏费用吗？'),
        answer: t(
          '正常情况下不会额外增加页面未展示的费用，但不同模型可能有不同计量口径，例如输入、输出、图片尺寸或任务时长。建议在调用前先查看对应模型说明。',
        ),
      },
      {
        id: 8,
        category: 'api',
        question: t('速率限制是什么？'),
        answer: t(
          '平台可能会对令牌、模型或用户维度设置请求频率限制，以保证整体稳定性。当请求过快时，接口可能返回 429，请适当重试并做好退避策略。',
        ),
      },
      {
        id: 9,
        category: 'api',
        question: t('如何验证API请求？'),
        answer: t(
          '所有 API 请求通常都需要在请求头中携带 `Authorization: Bearer YOUR_API_KEY`。同时建议设置 `Content-Type: application/json`，并确认 Base URL 配置正确。',
        ),
      },
      {
        id: 10,
        category: 'api',
        question: t('支持哪些端点？'),
        answer: t(
          '常见支持包括 `/v1/chat/completions`、`/v1/models`、`/v1/responses`、`/v1/images/generations`、`/v1/images/edits`、`/v1/videos` 以及 Gemini 兼容接口等。',
        ),
      },
      {
        id: 11,
        category: 'api',
        question: t('如何处理 API 错误？'),
        answer: t(
          '建议先检查模型名、请求体格式、鉴权头和额度状态。400 多为参数问题，401 多为鉴权失败，429 表示限流，500 则通常与上游或网关临时异常有关。',
        ),
      },
      {
        id: 12,
        category: 'account',
        question: t('如何重置密码？'),
        answer: t(
          '可以通过登录页的找回密码入口发起重置流程。如果站点启用了邮件验证，系统会向你的绑定邮箱发送重置邮件或验证码。',
        ),
      },
      {
        id: 13,
        category: 'account',
        question: t('如何修改邮箱？'),
        answer: t(
          '登录后可前往个人设置查看当前账户资料。若站点开放了邮箱绑定或修改功能，可以在那里完成变更；否则需要联系管理员协助处理。',
        ),
      },
      {
        id: 14,
        category: 'account',
        question: t('我的数据如何被保护？'),
        answer: t(
          '平台通常会通过令牌鉴权、权限隔离、传输加密以及操作日志等方式保护账户与调用数据。实际保护策略仍以当前站点的隐私政策与部署方式为准。',
        ),
      },
      {
        id: 15,
        category: 'account',
        question: t('我可以删除账户吗？'),
        answer: t(
          '如果站点开启了账户删除能力，你可以在个人设置中发起删除操作。删除前请确保你已备份需要保留的数据，因为部分记录可能无法恢复。',
        ),
      },
      {
        id: 16,
        category: 'technical',
        question: t('如何报告 Bug？'),
        answer: t(
          '如果你发现异常行为，建议整理好复现步骤、请求参数、错误码以及出现时间，再通过站点提供的反馈入口或支持邮箱提交给维护团队。',
        ),
      },
      {
        id: 17,
        category: 'technical',
        question: t('如果遇到性能问题怎么办？'),
        answer: t(
          '可先确认是否为单一模型波动、局部网络问题或请求参数过大导致。如果问题持续存在，建议附带请求时间、模型名和延迟表现联系技术支持。',
        ),
      },
      {
        id: 18,
        category: 'technical',
        question: t('有固定维护时间吗？'),
        answer: t(
          '维护策略由站点运营方决定。若有计划维护，通常会通过公告、通知或首页提示提前告知；紧急变更则可能在短时间内完成切换。',
        ),
      },
      {
        id: 19,
        category: 'integration',
        question: t('如何开始集成？'),
        answer: t(
          '优先使用兼容 OpenAI 的现成 SDK 或客户端，将平台地址替换为你的站点 Base URL，并填入 API Key。这样通常能以最小改动快速完成接入。',
        ),
      },
      {
        id: 20,
        category: 'integration',
        question: t('支持哪些 SDK？'),
        answer: t(
          '只要工具或 SDK 支持 OpenAI 风格接口，通常就可以较低成本接入。对于 Gemini 协议兼容场景，也可以直接使用对应的原生调用方式。',
        ),
      },
      {
        id: 21,
        category: 'integration',
        question: t('支持 Webhooks 吗？'),
        answer: t(
          '部分异步任务或业务扩展场景可以结合 Webhook 能力实现自动回调，但是否开放以及具体格式取决于当前站点与后端配置。',
        ),
      },
      {
        id: 22,
        category: 'integration',
        question: t('集成时也会受到速率限制吗？'),
        answer: t(
          '会。无论是手工调用还是通过第三方系统集成，只要使用同一令牌或同一模型能力，都可能受到相同的限流与并发约束。',
        ),
      },
    ],
    [systemName, t],
  );

  const filteredQuestions = useMemo(() => {
    return questions.filter((item) => {
      const categoryMatch =
        selectedCategory === 'all' || item.category === selectedCategory;
      const keyword = searchTerm.trim().toLowerCase();
      const searchMatch =
        keyword.length === 0 ||
        item.question.toLowerCase().includes(keyword) ||
        item.answer.toLowerCase().includes(keyword);
      return categoryMatch && searchMatch;
    });
  }, [questions, searchTerm, selectedCategory]);

  const activeCategoryName =
    categories.find((item) => item.id === selectedCategory)?.name || t('所有问题');

  return (
    <MarketingShell activeNav='faq' className='landing-faq'>
      <section className='landing-faq__hero'>
        <div className='landing-faq__heroInner'>
          <p className='landing-faq__eyebrow'>FAQ CENTER</p>
          <h1 className='landing-faq__title'>{t('常见问题解答')}</h1>
          <p className='landing-faq__subtitle'>
            {t(
              '找到您关于我们 AI 平台的常见问题答案。如果您找不到答案，请随时联系支持团队。',
            )}
          </p>

          <div className='landing-faq__searchWrap'>
            <div className='landing-faq__searchBox'>
              <input
                type='text'
                className='landing-faq__searchInput'
                placeholder={t('搜索问题...')}
                value={searchTerm}
                onChange={(event) => setSearchTerm(event.target.value)}
              />
              <span className='landing-faq__searchIcon'>
                <Search size={18} />
              </span>
            </div>
          </div>

          <div className='landing-faq__categories'>
            {categories.map((category) => (
              <button
                key={category.id}
                type='button'
                className={[
                  'landing-faq__pill',
                  selectedCategory === category.id ? 'landing-faq__pill--active' : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
                onClick={() => setSelectedCategory(category.id)}
              >
                <span className='landing-faq__pillIcon'>{category.icon}</span>
                <span>{category.name}</span>
              </button>
            ))}
          </div>
        </div>
      </section>

      <section className='landing-faq__listSection'>
        <div className='landing-faq__listInner'>
          <div className='landing-faq__listHead'>
            <h2 className='landing-faq__listTitle'>
              {selectedCategory === 'all' ? 'All Questions' : activeCategoryName}
            </h2>
            <p className='landing-faq__listMeta'>
              {filteredQuestions.length} questions found
              {searchTerm ? ` for "${searchTerm}"` : ''}
            </p>
          </div>

          <div className='landing-faq__items'>
            {filteredQuestions.length > 0 ? (
              filteredQuestions.map((item) => {
                const category = categories.find((entry) => entry.id === item.category);
                const open = openQuestionId === item.id;

                return (
                  <article
                    key={item.id}
                    className={[
                      'landing-faq__item',
                      open ? 'landing-faq__item--open' : '',
                    ]
                      .filter(Boolean)
                      .join(' ')}
                  >
                    <button
                      type='button'
                      className='landing-faq__question'
                      onClick={() => setOpenQuestionId(open ? null : item.id)}
                    >
                      <span className='landing-faq__questionLead'>
                        <span className='landing-faq__questionEmoji'>
                          {category?.icon}
                        </span>
                        <span className='landing-faq__questionText'>
                          {item.question}
                        </span>
                      </span>
                      <span
                        className={[
                          'landing-faq__toggle',
                          open ? 'landing-faq__toggle--open' : '',
                        ]
                          .filter(Boolean)
                          .join(' ')}
                      >
                        <ChevronDown size={18} />
                      </span>
                    </button>

                    {open && (
                      <div className='landing-faq__answer'>
                        <p>{item.answer}</p>
                      </div>
                    )}
                  </article>
                );
              })
            ) : (
              <div className='landing-faq__empty'>
                <div className='landing-faq__emptyIcon'>🔍</div>
                <h3>{t('没有找到匹配的问题')}</h3>
                <p>
                  {searchTerm
                    ? t('请尝试更换关键词，或切换到其他分类查看。')
                    : t('当前分类下暂时没有可展示的问题。')}
                </p>
                <div className='landing-faq__emptyActions'>
                  <button
                    type='button'
                    className='landing-faq__emptyButton'
                    onClick={() => {
                      setSearchTerm('');
                      setSelectedCategory('all');
                    }}
                  >
                    {t('查看全部问题')}
                  </button>
                  <Link to='/chat-api' className='landing-faq__emptyLink'>
                    {t('前往 API 文档')}
                  </Link>
                </div>
              </div>
            )}
          </div>
        </div>
      </section>

      {showBackTop && (
        <button
          type='button'
          className='landing-faq__backTop'
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        >
          ↑
        </button>
      )}
    </MarketingShell>
  );
};

export default Faq;
