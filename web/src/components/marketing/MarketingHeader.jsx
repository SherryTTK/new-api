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
import { BellDot } from 'lucide-react';
import { getLogo, getSystemName } from '../../helpers';

const MarketingHeader = ({ activeNav = 'home' }) => {
  const { t } = useTranslation();
  const systemName = getSystemName();
  const logo = getLogo();

  const navItems = [
    {
      key: 'home',
      label: t('首页'),
      render: (className) => <Link to='/' className={className}>{t('首页')}</Link>,
    },
    {
      key: 'pricing',
      label: t('模型'),
      render: (className) => (
        <Link to='/pricing' className={className}>
          {t('模型')}
        </Link>
      ),
    },
    {
      key: 'docs',
      label: t('API文档'),
      render: (className) => (
        <Link to='/chat-api' className={className}>
          {t('API文档')}
        </Link>
      ),
    },
    {
      key: 'faq',
      label: t('常见问题'),
      render: (className) => (
        <Link to='/faq' className={className}>
          {t('常见问题')}
        </Link>
      ),
    },
    {
      key: 'buy',
      label: t('购买'),
      render: (className) => (
        <Link to='/purchase' className={className}>
          {t('购买')}
        </Link>
      ),
    },
  ];

  return (
    <header className='landing-home__header'>
      <div className='landing-home__headerInner'>
        <Link to='/' className='landing-home__brand'>
          <img src={logo} alt={systemName} className='landing-home__brandLogo' />
          <span className='landing-home__brandText'>{systemName}</span>
        </Link>

        <nav className='landing-home__nav'>
          {navItems.map((item) => {
            const className = [
              'landing-home__navItem',
              activeNav === item.key ? 'landing-home__navItem--active' : '',
            ]
              .filter(Boolean)
              .join(' ');
            return <React.Fragment key={item.key}>{item.render(className)}</React.Fragment>;
          })}
        </nav>

        <div className='landing-home__headerActions'>
          <button
            type='button'
            className='landing-home__headerIconButton'
            aria-label={t('通知')}
          >
            <BellDot size={16} />
          </button>
          <Link
            to='/login'
            className={[
              'landing-home__loginButton',
              activeNav === 'login' ? 'landing-home__loginButton--active' : '',
            ]
              .filter(Boolean)
              .join(' ')}
          >
            {t('登录')}
          </Link>
          <Link
            to='/register'
            className={[
              'landing-home__registerButton',
              activeNav === 'register'
                ? 'landing-home__registerButton--active'
                : '',
            ]
              .filter(Boolean)
              .join(' ')}
          >
            {t('注册')}
          </Link>
        </div>
      </div>
    </header>
  );
};

export default MarketingHeader;
