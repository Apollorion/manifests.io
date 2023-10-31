import {NextSeo} from 'next-seo';

const url = "https://www.manifests.io";

type Props = {
    item: string;
    version: string;
    description: string;
    resource?: string;
};

const ManifestsHeading = ({item, version, description, resource}: Props) => (
    <NextSeo
        title="Manifests.io"
        description="Easy Kubernetes Documentation!"
        canonical={url}
        openGraph={{
            url: resource ? `${url}/${item}/${version}/${resource}` : `${url}/${item}/${version}`,
            title: `Easy documentation for ${item} v${version}.`,
            description: description !== "" ? description : "Easy Kubernetes Documentation!",
            images: [
                {
                    url: `${url}/ogimage.png`,
                    width: 887,
                    height: 465,
                    alt: 'Manifests.io',
                    type: 'image/png',
                }
            ],
            site_name: 'Manifests.io',
        }}
        additionalLinkTags={[
            {
                rel: 'icon',
                href: `${url}/favicon.ico`,
            },
            {
                rel: 'apple-touch-icon',
                href: `${url}/apple-touch-icon.png`,
                sizes: '76x76'
            },
            {
                rel: 'manifest',
                href: `${url}/manifest.json`
            }
        ]}
    />
);

export default ManifestsHeading;