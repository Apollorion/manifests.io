export interface Product {
  name: string;
  versions: string[];
}

export interface Link {
  label: string;
  href: string;
}

export interface Row {
  name: string;
  type: string;
  description: string;
  required?: boolean;
  href?: string;
  constraints?: string[];
  variants?: Link[];
}

export interface Page {
  item: string;
  version: string;
  resource?: string;
  title: string;
  description: string;
  path?: string;
  pointer?: string;
  resources: Row[];
  otherVersions: Link[];
  breadcrumbs: Link[];
  variants: Link[];
  catalog: Product[];
  canonical: string;
  error?: string;
}
