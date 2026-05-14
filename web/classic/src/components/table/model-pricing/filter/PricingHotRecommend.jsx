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
import { getLobeHubIcon } from '../../../../helpers';

/**
 * 热门推荐组件 - 显示在筛选栏顶部
 * 展示热门/推荐的供应商，点击可快速筛选
 *
 * @param {Array} models 模型列表
 * @param {string} filterVendor 当前选中的供应商
 * @param {Function} setFilterVendor 设置供应商筛选
 * @param {boolean} loading 是否加载中
 * @param {Function} t i18n
 */
const PricingHotRecommend = ({
  filterVendor,
  setFilterVendor,
  t,
}) => {
  const handleClick = () => {
    if (filterVendor === '腾讯') {
      setFilterVendor('all');
    } else {
      setFilterVendor('腾讯');
    }
  };

  const isActive = filterVendor === '腾讯';

  return (
    <div className='mb-6'>
      {/* 热门推荐卡片 */}
      <div
        style={{
          background: 'linear-gradient(90deg, #8b72fc, #E5A2FD 100%)',
          borderRadius: '10px',
          paddingLeft: '32px',
          height: '93px',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          marginBottom: '8px',
        }}
      >
        <div className='flex items-center gap-[2px] mb-[8px]'>
          <img src="/hot-recommend.svg" className='w-[19px] h-[19px]' />
          <span style={{ color: '#fff', fontSize: '16px', fontWeight: 600 }}>
            {t('热门推荐')}
          </span>
        </div>

        {/* 腾讯按钮 */}
        <div
          onClick={handleClick}
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            width: '197px', 
            padding: '0 19px',
            height: '41px',
            borderRadius: '10px',
            background: isActive ? '#6639BF' : '#ede3ff',
            color: isActive ? '#fff' : 'rgba(28,31,35,0.8)',
            cursor: 'pointer',
            fontSize: '14px',
            fontWeight: 600,
            transition: 'all 0.15s ease',
            border: '1px solid rgba(182,170,255,0.55)',
          }}
          onMouseEnter={(e) => {
            if (!isActive) {
              e.currentTarget.style.background = '#6639BF';
              e.currentTarget.style.color = '#fff';
            }
          }}
          onMouseLeave={(e) => {
            if (!isActive) {
              e.currentTarget.style.background = '#ede3ff';
              e.currentTarget.style.color = 'rgba(28,31,35,0.8)';
            }
          }}
        >
            <div style={{ fontSize: '16px', lineHeight: 1 }}>
                {getLobeHubIcon('Hunyuan.Color', 16)}
            </div>
            <div style={{ flex: 1, marginLeft: '5px' }}>{t('腾讯')}</div>
            <div
                style={{
                    width: '37px', 
                    height: '13px', 
                    display: 'flex',
                    justifyContent: 'center',
                    alignItems: 'center',
                    borderRadius: '20px',  
                    marginLeft: '5px', 
                    backgroundColor: '#8651EE', 
                    fontSize: '11px', 
                    color: '#fff'
                }}
            >
                HOT
            </div>
        </div>
      </div>
    </div>
  );
};

export default PricingHotRecommend;
