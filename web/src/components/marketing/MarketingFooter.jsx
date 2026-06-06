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

import React from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Mail, MapPin, Phone } from 'lucide-react';
import { getLogo, getSystemName } from '../../helpers';

const MarketingFooter = () => {
  const { t } = useTranslation();
  const systemName = getSystemName();
  const logo = getLogo();
  const currentYear = new Date().getFullYear();

  const footerColumns = [
    {
      title: t('联系我们'),
      items: [
        { icon: MapPin, text: t('浙江省 杭州市 余杭区') },
        { icon: Mail, text: 'support@example.com' },
        { icon: Phone, text: '+86 13212345125（工作日 9:00-18:00）' },
      ],
    },
    {
      title: t('产品索引'),
      items: [
        { text: t('价格/计费'), to: '/purchase' },
        { text: t('模型介绍'), to: '/pricing' },
        { text: t('API 文档'), to: '/chat-api' },
      ],
    },
    {
      title: t('支持导航'),
      items: [
        { text: t('常见问题'), to: '/faq' },
        { text: t('隐私政策'), to: '/privacy-policy' },
        { text: t('服务条款'), to: '/user-agreement' },
      ],
    },
  ];

  return (
    <footer className='landing-home__footer'>
      <div className='landing-home__footerGrid'>
        <div className='landing-home__footerIntro'>
          <div className='landing-home__brand landing-home__brand--footer'>
            <img src={logo} alt={systemName} className='landing-home__brandLogo' />
            <span className='landing-home__brandText'>{systemName}</span>
          </div>
          <p>
            {t(
              '以统一 API 入口聚合全球智能，为企业与创作者提供稳定、弹性、低成本的 AI 调度体验。',
            )}
          </p>
          <span className='landing-home__footerNote'>
            {`${systemName}${t('，让连接世界 AI 的距离更短。')}`}
          </span>
        </div>

        {footerColumns.map((column) => (
          <div key={column.title} className='landing-home__footerColumn'>
            <h3>{column.title}</h3>
            <div className='landing-home__footerItems'>
              {column.items.map((item) => (
                <div key={item.text} className='landing-home__footerItem'>
                  {item.icon && <item.icon size={15} />}
                  {item.to ? (
                    <Link to={item.to} className='landing-home__footerLink'>
                      {item.text}
                    </Link>
                  ) : (
                    <span>{item.text}</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      <div className='landing-home__footerBottom'>
        <span>
          © {currentYear} {systemName}
        </span>
        <span>{t('由全球模型聚合能力驱动')}</span>
      </div>
    </footer>
  );
};

export default MarketingFooter;
