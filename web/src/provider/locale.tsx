'use client';

import { type ReactNode } from 'react';
import { NextIntlClientProvider } from 'next-intl';
import { useSettingStore, type Locale } from '@/stores/setting';

import zh_hansMessages from '../../public/locale/zh_hans.json';
import zh_hantMessages from '../../public/locale/zh_hant.json';
import enMessages from '../../public/locale/en.json';

const messages: Record<Locale, typeof zh_hansMessages> = {
    'zh-Hans': zh_hansMessages,
    'zh-Hant': zh_hantMessages,
    en: enMessages,
};

// Map internal locale codes to BCP 47 language tags
// function toBCP47Locale(locale: Locale): string {
//     const localeMap: Record<Locale, string> = {
//         zh_hans: 'zh-Hans',
//         zh_hant: 'zh-Hant',
//         en: 'en',
//     };
//     return localeMap[locale];
// }

export function LocaleProvider({ children }: { children: ReactNode }) {
    const { locale } = useSettingStore();

    return (
        <NextIntlClientProvider
            locale={locale}
            messages={messages[locale]}
            timeZone="Asia/Shanghai"
        >
            {children}
        </NextIntlClientProvider>
    );
}

