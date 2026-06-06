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
import './marketing.css';
import MarketingHeader from './MarketingHeader';
import MarketingFooter from './MarketingFooter';

const MarketingShell = ({ activeNav = 'home', className = '', children }) => {
  const rootClassName = ['landing-home', className].filter(Boolean).join(' ');

  return (
    <div className={rootClassName}>
      <div className='landing-home__glow landing-home__glow--left' />
      <div className='landing-home__glow landing-home__glow--right' />
      <div className='landing-home__glow landing-home__glow--bottom' />
      <div className='landing-home__noise' />

      <MarketingHeader activeNav={activeNav} />
      <main className='landing-home__main'>{children}</main>
      <MarketingFooter />
    </div>
  );
};

export default MarketingShell;
