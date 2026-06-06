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
import { CalendarDays, FileLock2, Mail, ShieldCheck } from 'lucide-react';
import MarketingShell from './MarketingShell';

const defaultSignals = [
  { icon: FileLock2, label: '隐私优先' },
  { icon: ShieldCheck, label: '合规存储' },
  { icon: Mail, label: '15 个工作日内回复' },
];

const defaultQuickLinks = [
  { label: '返回首页', to: '/' },
  { label: '常见问题', to: '/faq' },
  { label: 'API 文档', to: '/chat-api' },
];

const LegalDocumentPage = ({
  title,
  eyebrow,
  effectiveDate,
  introParagraphs,
  sections,
  signals = defaultSignals,
  quickLinks = defaultQuickLinks,
  copyright = 'Copyright Yuanshu 元枢AI All Rights Reserved',
}) => {
  return (
    <MarketingShell activeNav='home' className='landing-privacy'>
      <section className='landing-privacy__hero'>
        <div className='landing-privacy__heroHeader'>
          <div>
            <p className='landing-privacy__eyebrow'>{eyebrow}</p>
            <h1 className='landing-privacy__title'>{title}</h1>
            <div className='landing-privacy__meta'>
              <span className='landing-privacy__metaItem'>
                <CalendarDays size={15} />
                {effectiveDate}
              </span>
            </div>
          </div>

          <div className='landing-privacy__signalRow'>
            {signals.map((signal) => {
              const Icon = signal.icon;
              return (
                <span key={signal.label} className='landing-privacy__signal'>
                  <Icon size={14} />
                  {signal.label}
                </span>
              );
            })}
          </div>
        </div>
      </section>

      <section className='landing-privacy__content'>
        <div className='landing-privacy__panel'>
          <div className='landing-privacy__document'>
            {introParagraphs.map((paragraph) => (
              <p key={paragraph} className='landing-privacy__paragraph'>
                {paragraph}
              </p>
            ))}

            {sections.map((section) => (
              <section key={section.title} className='landing-privacy__section'>
                <h2 className='landing-privacy__sectionTitle'>{section.title}</h2>

                {section.paragraphs?.map((paragraph) => (
                  <p key={paragraph} className='landing-privacy__paragraph'>
                    {paragraph}
                  </p>
                ))}

                {section.items?.length > 0 && (
                  <ol className='landing-privacy__list'>
                    {section.items.map((item) => (
                      <li key={item} className='landing-privacy__listItem'>
                        {item}
                      </li>
                    ))}
                  </ol>
                )}

                {section.groups?.map((group) => (
                  <div key={group.title} className='landing-privacy__group'>
                    <h3 className='landing-privacy__groupTitle'>{group.title}</h3>
                    <ol className='landing-privacy__list'>
                      {group.items.map((item) => (
                        <li key={item} className='landing-privacy__listItem'>
                          {item}
                        </li>
                      ))}
                    </ol>
                  </div>
                ))}
              </section>
            ))}

            <div className='landing-privacy__ending'>
              <span>{copyright}</span>
            </div>
          </div>
        </div>

        <div className='landing-privacy__quickLinks'>
          {quickLinks.map((link) => (
            <Link key={link.to} to={link.to} className='landing-privacy__quickLink'>
              {link.label}
            </Link>
          ))}
        </div>
      </section>
    </MarketingShell>
  );
};

export default LegalDocumentPage;
