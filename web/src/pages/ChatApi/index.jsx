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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  ArrowUpRight,
  Bot,
  Copy,
  FileJson,
  Image,
  KeyRound,
  Sparkles,
  Video,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import MarketingShell from '../../components/marketing/MarketingShell';
import MarkdownRenderer from '../../components/common/markdown/MarkdownRenderer';
import { StatusContext } from '../../context/Status';
import {
  copy,
  getSystemName,
  removeTrailingSlash,
  showSuccess,
} from '../../helpers';

const ChatApi = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const [activeSection, setActiveSection] = useState('introduction');
  const [showBackTop, setShowBackTop] = useState(false);
  const systemName = getSystemName();

  const status = useMemo(() => {
    if (statusState?.status) return statusState.status;
    const savedStatus = localStorage.getItem('status');
    if (!savedStatus) return {};
    try {
      return JSON.parse(savedStatus) || {};
    } catch (error) {
      return {};
    }
  }, [statusState?.status]);

  const serverAddress = useMemo(() => {
    const fallback =
      typeof window !== 'undefined' ? window.location.origin : '';
    return removeTrailingSlash(status?.server_address || fallback);
  }, [status?.server_address]);

  const baseUrl = `${serverAddress}/v1`;
  const chatEndpoint = `${baseUrl}/chat/completions`;
  const modelsEndpoint = `${baseUrl}/models`;
  const responsesEndpoint = `${baseUrl}/responses`;
  const compactResponsesEndpoint = `${baseUrl}/responses/compact`;
  const imagesEndpoint = `${baseUrl}/images/generations`;
  const imageEditEndpoint = `${baseUrl}/images/edits`;
  const videosEndpoint = `${baseUrl}/videos`;
  const geminiModelsEndpoint = `${serverAddress}/v1beta/models`;

  const quickLinks = useMemo(
    () => [
      {
        icon: Bot,
        title: 'Chat Completions',
        description: '对话、多轮上下文、工具调用、流式输出',
        method: 'POST',
        path: '/v1/chat/completions',
        value: chatEndpoint,
      },
      {
        icon: Image,
        title: 'Images',
        description: '文生图与图像编辑，兼容 OpenAI 风格请求',
        method: 'POST',
        path: '/v1/images/generations',
        value: imagesEndpoint,
      },
      {
        icon: Video,
        title: 'Videos',
        description: 'OpenAI 兼容视频创建、查询与内容回传',
        method: 'POST',
        path: '/v1/videos',
        value: videosEndpoint,
      },
      {
        icon: Sparkles,
        title: 'Gemini',
        description: '保留 Gemini 原生协议，可直接接入现有客户端',
        method: 'GET',
        path: '/v1beta/models',
        value: geminiModelsEndpoint,
      },
    ],
    [chatEndpoint, geminiModelsEndpoint, imagesEndpoint, videosEndpoint],
  );

  const sections = useMemo(
    () => [
      {
        id: 'introduction',
        title: '📝 简介',
        content: [
          '## 接口概览',
          '',
          `${systemName} 提供兼容 OpenAI Chat Completions 的统一 AI 接口。你可以使用一套鉴权方式与一组基础地址，同时访问 GPT、Claude、Gemini、图像生成、视频生成等能力。`,
          '',
          '## 核心特性',
          '',
          '- 完全兼容 OpenAI Chat Completions API',
          '- 支持多模态输入（文本 + 图片）',
          '- 支持实时流式响应',
          '- 支持函数调用与工具使用',
          '- 保留 Gemini 原生协议入口，便于兼容已有生态',
          '',
          '## 常用兼容端点',
          '',
          `- \`GET ${modelsEndpoint}\` 获取模型列表`,
          `- \`POST ${chatEndpoint}\` 发起对话补全`,
          `- \`POST ${responsesEndpoint}\` 使用 OpenAI Responses 接口`,
          `- \`POST ${compactResponsesEndpoint}\` 使用紧凑型 Responses 接口`,
        ].join('\n'),
      },
      {
        id: 'api-access',
        title: '🔑 API获取',
        content: [
          '## API 访问',
          '',
          `API 基础地址：\`${baseUrl}\``,
          '',
          '所有请求都需要携带 API Key，并使用标准 Bearer Token 鉴权：',
          '',
          '```http',
          'Authorization: Bearer YOUR_API_KEY',
          'Content-Type: application/json',
          '```',
          '',
          '创建令牌后，将客户端中的 Base URL 设置为上方地址即可接入。常见的 OpenAI SDK、聊天客户端和工作流工具都可以直接使用。',
          '',
          '令牌管理入口：`/console/token`',
          '',
          `如果你需要先体验价格与模型列表，也可以从首页进入“模型”页面查看当前可用能力，再回到 ${systemName} 控制台创建令牌。`,
        ].join('\n'),
      },
      {
        id: 'examples',
        title: '💡 请求示例',
        content: [
          '### 文字聊天',
          '',
          '```bash',
          `curl ${chatEndpoint} \\`,
          '  -H "Authorization: Bearer YOUR_API_KEY" \\',
          '  -H "Content-Type: application/json" \\',
          "  -d '{",
          '    "model": "gpt-4o-mini",',
          '    "temperature": 0.6,',
          '    "max_tokens": 600,',
          '    "messages": [',
          '      {',
          '        "role": "system",',
          '        "content": "你是一个擅长商业分析的助手。"',
          '      },',
          '      {',
          '        "role": "user",',
          '        "content": "请帮我总结今天的市场动态，输出三条要点。"',
          '      }',
          '    ]',
          "  }'",
          '```',
          '',
          '### 多模态图像分析',
          '',
          '```json',
          '{',
          '  "model": "gpt-4o-mini",',
          '  "messages": [',
          '    {',
          '      "role": "user",',
          '      "content": [',
          '        {',
          '          "type": "text",',
          '          "text": "请概括这张图片里的主要内容，并指出值得关注的细节。"',
          '        },',
          '        {',
          '          "type": "image_url",',
          '          "image_url": {',
          '            "url": "https://example.com/demo.png"',
          '          }',
          '        }',
          '      ]',
          '    }',
          '  ],',
          '  "stream": true',
          '}',
          '```',
        ].join('\n'),
      },
      {
        id: 'image-generation',
        title: '🖼️ 图片生成',
        content: [
          '## 文生图',
          '',
          `图片生成端点：\`POST ${imagesEndpoint}\``,
          '',
          '```bash',
          `curl ${imagesEndpoint} \\`,
          '  -H "Authorization: Bearer YOUR_API_KEY" \\',
          '  -H "Content-Type: application/json" \\',
          "  -d '{",
          '    "model": "gpt-image-1",',
          '    "prompt": "生成一张未来城市夜景海报，玻璃幕墙反射蓝绿色霓虹光，电影感构图。",',
          '    "size": "1024x1024"',
          "  }'",
          '```',
          '',
          '返回结构兼容 OpenAI Images 风格，通常会在 `data` 数组中返回图片地址或 Base64 内容，具体取决于上游模型能力与配置。',
        ].join('\n'),
      },
      {
        id: 'image-to-image',
        title: '🖼️ 图生图',
        content: [
          '## 图片编辑',
          '',
          `图像编辑端点：\`POST ${imageEditEndpoint}\``,
          '',
          '当上游模型支持图像编辑时，可以上传原图并结合 prompt 进行修改。常见场景包括背景替换、风格迁移、局部重绘等。',
          '',
          '```bash',
          `curl ${imageEditEndpoint} \\`,
          '  -H "Authorization: Bearer YOUR_API_KEY" \\',
          '  -F "model=gpt-image-1" \\',
          '  -F "image=@input.png" \\',
          '  -F "prompt=保留主体，把背景改成雨夜霓虹街头，整体风格更有赛博朋克氛围"',
          '```',
        ].join('\n'),
      },
      {
        id: 'video-generation',
        title: '📹 OpenAI Videos 创建接口',
        content: [
          '## 视频生成',
          '',
          'OpenAI 兼容视频接口适合异步任务流：先创建任务，再轮询状态，最后获取视频内容。',
          '',
          `- 创建任务：\`POST ${videosEndpoint}\``,
          `- 查询任务：\`GET ${videosEndpoint}/{task_id}\``,
          `- 获取内容：\`GET ${videosEndpoint}/{task_id}/content\``,
          '',
          '```bash',
          `curl ${videosEndpoint} \\`,
          '  -H "Authorization: Bearer YOUR_API_KEY" \\',
          '  -H "Content-Type: application/json" \\',
          "  -d '{",
          '    "model": "YOUR_VIDEO_MODEL",',
          '    "prompt": "一只机械鲸鱼穿过霓虹城市上空，镜头从广角缓缓推进，时长 8 秒。"',
          "  }'",
          '```',
          '',
          '多数视频模型会先返回任务 ID。任务完成后，可以继续请求状态接口，并在需要时通过 `/content` 路径拉取最终视频文件。',
        ].join('\n'),
      },
      {
        id: 'request',
        title: '📮 请求参数',
        content: [
          '## Chat Completions 请求体',
          '',
          '| 字段 | 类型 | 必填 | 说明 |',
          '| --- | --- | --- | --- |',
          '| `model` | string | 是 | 要调用的模型名称 |',
          '| `messages` | array | 是 | 对话消息数组，支持多轮上下文 |',
          '| `temperature` | number | 否 | 采样温度，控制输出发散程度 |',
          '| `top_p` | number | 否 | 核采样参数，通常与 `temperature` 二选一调节 |',
          '| `max_tokens` | number | 否 | 控制单次返回的最大 token 数量 |',
          '| `stream` | boolean | 否 | 是否开启流式返回 |',
          '| `tools` | array | 否 | 函数调用或工具声明列表 |',
          '| `response_format` | object | 否 | 控制输出结构，例如 JSON 模式 |',
          '',
          '## 认证说明',
          '',
          '每次调用都需要使用 Bearer Token。你也可以先通过 `GET /v1/models` 验证令牌是否生效，再发起正式请求。',
        ].join('\n'),
      },
      {
        id: 'response',
        title: '📥 响应格式',
        content: [
          '## 标准返回结构',
          '',
          '```json',
          '{',
          '  "id": "chatcmpl-abc123",',
          '  "object": "chat.completion",',
          '  "created": 1716800000,',
          '  "model": "gpt-4o-mini",',
          '  "choices": [',
          '    {',
          '      "index": 0,',
          '      "message": {',
          '        "role": "assistant",',
          '        "content": "以下是今天的三条市场动态要点..."',
          '      },',
          '      "finish_reason": "stop"',
          '    }',
          '  ],',
          '  "usage": {',
          '    "prompt_tokens": 89,',
          '    "completion_tokens": 162,',
          '    "total_tokens": 251',
          '  }',
          '}',
          '```',
          '',
          '当 `stream=true` 时，接口会返回与 OpenAI SSE 兼容的分块事件流，可以直接用于前端流式渲染。',
        ].join('\n'),
      },
      {
        id: 'parameters',
        title: '⚙️ 参数详解',
        content: [
          '## 常见参数建议',
          '',
          '| 参数 | 典型值 | 说明 |',
          '| --- | --- | --- |',
          '| `temperature` | `0.2 - 0.8` | 越低越稳定，越高越发散 |',
          '| `top_p` | `0.8 - 1.0` | 替代或辅助 `temperature` 使用 |',
          '| `max_tokens` | `256 / 512 / 1024+` | 视回答长度与成本预算设置 |',
          '| `stream` | `true` / `false` | 需要打字机效果或低感知延迟时建议开启 |',
          '| `tools` | 数组 | 让模型以结构化方式调用函数 |',
          '| `response_format` | `json_schema` / `json_object` | 需要稳定结构化输出时使用 |',
          '| `logprobs` | `true` | 仅在对应模型支持时有效 |',
          '',
          '## 实战建议',
          '',
          '- 做客服、知识库问答时，优先降低 `temperature`。',
          '- 做头脑风暴、创意生成时，可以适当提高 `temperature`。',
          '- 流式输出适合聊天界面，批处理任务则更适合非流式请求。',
          '- 对结构化结果有要求时，请配合 `response_format` 或 `tools` 一起使用。',
        ].join('\n'),
      },
      {
        id: 'gemini-protocol',
        title: '⚡ Gemini 协议',
        content: [
          '## 原生 Gemini 兼容入口',
          '',
          `- \`GET ${geminiModelsEndpoint}\``,
          `- \`POST ${geminiModelsEndpoint}/{model}:generateContent\``,
          `- \`POST ${geminiModelsEndpoint}/{model}:streamGenerateContent?alt=sse\``,
          '',
          '如果你现有的客户端或代码已经按 Gemini 协议实现，可以直接保留调用方式，只需要将基础地址和密钥切换到本系统即可。',
          '',
          '```bash',
          `curl ${geminiModelsEndpoint}/gemini-2.0-flash:generateContent \\`,
          '  -H "Authorization: Bearer YOUR_API_KEY" \\',
          '  -H "Content-Type: application/json" \\',
          "  -d '{",
          '    "contents": [',
          '      {',
          '        "parts": [',
          '          {',
          '            "text": "请用三点总结为什么统一网关适合团队开发。"',
          '          }',
          '        ]',
          '      }',
          '    ]',
          "  }'",
          '```',
        ].join('\n'),
      },
      {
        id: 'error-handling',
        title: '🚨 错误处理',
        content: [
          '## 常见状态码',
          '',
          '| 状态码 | 含义 | 常见原因 |',
          '| --- | --- | --- |',
          '| `400` | 请求参数错误 | JSON 格式不合法、字段缺失、模型名不正确 |',
          '| `401` | 鉴权失败 | API Key 缺失、无效或已禁用 |',
          '| `429` | 请求过多 | 触发令牌限流、模型限流或上游额度限制 |',
          '| `500` | 服务异常 | 上游模型错误、代理链路异常、内部服务故障 |',
          '',
          '## 错误响应示例',
          '',
          '```json',
          '{',
          '  "error": {',
          '    "message": "Invalid API key provided",',
          '    "type": "invalid_request_error",',
          '    "code": "unauthorized"',
          '  }',
          '}',
          '```',
          '',
          '当你排查问题时，建议优先检查 Base URL、模型名、Authorization 请求头和令牌状态是否正确。',
        ].join('\n'),
      },
    ],
    [
      baseUrl,
      chatEndpoint,
      compactResponsesEndpoint,
      geminiModelsEndpoint,
      imageEditEndpoint,
      imagesEndpoint,
      modelsEndpoint,
      responsesEndpoint,
      serverAddress,
      systemName,
      videosEndpoint,
    ],
  );

  useEffect(() => {
    const updateStateByScroll = () => {
      setShowBackTop(window.scrollY > 420);

      let currentId = sections[0]?.id || 'introduction';
      sections.forEach((section) => {
        const element = document.getElementById(section.id);
        if (!element) {
          return;
        }
        const { top } = element.getBoundingClientRect();
        if (top <= 180) {
          currentId = section.id;
        }
      });
      setActiveSection(currentId);
    };

    updateStateByScroll();
    window.addEventListener('scroll', updateStateByScroll, { passive: true });
    return () => window.removeEventListener('scroll', updateStateByScroll);
  }, [sections]);

  useEffect(() => {
    const hash = decodeURIComponent(window.location.hash.replace(/^#/, ''));
    if (!hash) {
      return;
    }
    const timer = window.setTimeout(() => {
      const target = document.getElementById(hash);
      if (target) {
        target.scrollIntoView({ behavior: 'smooth', block: 'start' });
        setActiveSection(hash);
      }
    }, 120);

    return () => window.clearTimeout(timer);
  }, [sections]);

  const handleCopy = async (text, message = '已复制到剪贴板') => {
    if (await copy(text)) {
      showSuccess(t(message));
    }
  };

  const handleScrollToSection = (sectionId) => {
    const target = document.getElementById(sectionId);
    if (!target) {
      return;
    }
    window.history.replaceState(null, '', `#${sectionId}`);
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setActiveSection(sectionId);
  };

  return (
    <MarketingShell activeNav='docs' className='landing-chat-api'>
      <section className='landing-chat-api__hero'>
        <div className='landing-chat-api__heroGrid'>
          <div className='landing-chat-api__heroContent'>
            <p className='landing-chat-api__eyebrow'>API DOCUMENTATION</p>
            <h1 className='landing-chat-api__title'>
              <span>💬 OpenAI Chat Completions</span>
              <span className='landing-chat-api__titleAccent'>
                {t('统一兼容接入')}
              </span>
            </h1>
            <p className='landing-chat-api__subtitle'>
              {t(
                '完全兼容 OpenAI Chat Completions API，支持对话、图像分析、函数调用、图片生成与视频生成等能力。',
              )}
            </p>

            <div className='landing-chat-api__badges'>
              <span className='landing-chat-api__badge'>✅ 基础文本对话</span>
              <span className='landing-chat-api__badge'>✅ 图像分析对话</span>
              <span className='landing-chat-api__badge'>✅ 流式响应</span>
              <span className='landing-chat-api__badge'>✅ 函数调用</span>
              <span className='landing-chat-api__badge'>✅ 视频生成</span>
            </div>

            <div className='landing-chat-api__actions'>
              <Link to='/console/token' className='landing-chat-api__primaryAction'>
                <KeyRound size={16} />
                {t('创建 API 令牌')}
              </Link>
              <Link to='/pricing' className='landing-chat-api__secondaryAction'>
                {t('查看模型价格')}
                <ArrowUpRight size={16} />
              </Link>
            </div>
          </div>

          <div className='landing-chat-api__heroSide'>
            <div className='landing-chat-api__endpointCard'>
              <div className='landing-chat-api__endpointMeta'>
                <span className='landing-chat-api__endpointMethod'>POST</span>
                <span className='landing-chat-api__endpointNote'>
                  请求头：Content-Type: application/json
                </span>
              </div>
              <code className='landing-chat-api__endpointCode'>{chatEndpoint}</code>
              <button
                type='button'
                className='landing-chat-api__copyButton'
                onClick={() => handleCopy(chatEndpoint, '接口地址已复制')}
              >
                <Copy size={15} />
                {t('复制地址')}
              </button>
            </div>
          </div>
        </div>

        <div className='landing-chat-api__quickGrid'>
          {quickLinks.map((item) => {
            const Icon = item.icon;
            return (
              <article key={item.title} className='landing-chat-api__quickCard'>
                <div className='landing-chat-api__quickHeader'>
                  <span className='landing-chat-api__quickIcon'>
                    <Icon size={18} />
                  </span>
                  <div>
                    <h2>{item.title}</h2>
                    <p>{item.description}</p>
                  </div>
                </div>

                <div className='landing-chat-api__quickFooter'>
                  <div className='landing-chat-api__quickPathWrap'>
                    <span className='landing-chat-api__quickMethod'>{item.method}</span>
                    <code>{item.path}</code>
                  </div>
                  <button
                    type='button'
                    className='landing-chat-api__quickCopy'
                    onClick={() => handleCopy(item.value)}
                    aria-label={`${item.title} copy`}
                  >
                    <Copy size={14} />
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      </section>

      <section className='landing-chat-api__workspace'>
        <aside className='landing-chat-api__sidebar'>
          <div className='landing-chat-api__sidebarCard'>
            <div className='landing-chat-api__sidebarHeader'>
              <FileJson size={16} />
              <span>{t('目录导航')}</span>
            </div>
            <div className='landing-chat-api__sidebarList'>
              {sections.map((section) => (
                <button
                  type='button'
                  key={section.id}
                  className={[
                    'landing-chat-api__sidebarLink',
                    activeSection === section.id
                      ? 'landing-chat-api__sidebarLink--active'
                      : '',
                  ]
                    .filter(Boolean)
                    .join(' ')}
                  onClick={() => handleScrollToSection(section.id)}
                >
                  {section.title}
                </button>
              ))}
            </div>
          </div>
        </aside>

        <div className='landing-chat-api__content'>
          {sections.map((section) => (
            <section
              id={section.id}
              key={section.id}
              className='landing-chat-api__section'
            >
              <div className='landing-chat-api__sectionCard'>
                <div className='landing-chat-api__sectionTop'>
                  <div>
                    <p className='landing-chat-api__sectionLabel'>
                      {t('文档章节')}
                    </p>
                    <h2 className='landing-chat-api__sectionTitle'>
                      {section.title}
                    </h2>
                  </div>

                  <button
                    type='button'
                    className='landing-chat-api__anchorButton'
                    onClick={() => handleScrollToSection(section.id)}
                  >
                    #{section.id}
                  </button>
                </div>

                <MarkdownRenderer
                  content={section.content}
                  className='landing-chat-api__markdown'
                />
              </div>
            </section>
          ))}
        </div>
      </section>

      {showBackTop && (
        <button
          type='button'
          className='landing-chat-api__backTop'
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        >
          ↑
        </button>
      )}
    </MarketingShell>
  );
};

export default ChatApi;
