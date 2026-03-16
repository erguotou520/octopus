'use client';

import { type ReactNode } from 'react';
import { NextIntlClientProvider } from 'next-intl';
import { useSettingStore, type Locale } from '@/stores/setting';

import zh_HansMessages from '../../public/locale/zh-Hans.json';
import zh_HantMessages from '../../public/locale/zh-Hant.json';
import enMessages from '../../public/locale/en.json';

const messages: Record<Locale, typeof zh_HansMessages> = {
    'zh-Hans': zh_HansMessages,
    'zh-Hant': zh_HantMessages,
    en: enMessages,
};

// Map internal locale codes to BCP 47 language tags
// function toBCP47Locale(locale: Locale): string {
//     const localeMap: Record<Locale, string> = {
//         zh-Hans: 'zh-Hans',
//         zh-Hant: 'zh-Hant',
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

