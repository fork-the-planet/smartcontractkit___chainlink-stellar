use proc_macro::TokenStream;
use quote::quote;
use std::collections::HashSet;
use syn::parse::Parser;
use syn::punctuated::Punctuated;
use syn::{parse_macro_input, Data, DeriveInput, Fields, Token, Type};

#[proc_macro_derive(ErrorConversions, attributes(from))]
pub fn derive_error_conversions(input: TokenStream) -> TokenStream {
    let input = parse_macro_input!(input as DeriveInput);

    let enum_name = &input.ident;
    let (impl_generics, ty_generics, where_clause) = input.generics.split_for_impl();

    let Data::Enum(data_enum) = &input.data else {
        return syn::Error::new_spanned(
            enum_name,
            "ErrorConversions can only be derived for enums",
        )
        .to_compile_error()
        .into();
    };

    let mut seen_sources = HashSet::new();
    let mut generated_impls = Vec::new();

    for variant in &data_enum.variants {
        let variant_name = &variant.ident;
        let has_from_attr = variant
            .attrs
            .iter()
            .any(|attr| attr.path().is_ident("from"));

        if has_from_attr && !matches!(variant.fields, Fields::Unit) {
            return syn::Error::new_spanned(
                variant,
                "#[from(...)] is only supported on unit enum variants",
            )
            .to_compile_error()
            .into();
        }

        for attr in &variant.attrs {
            if !attr.path().is_ident("from") {
                continue;
            }

            let meta_list = match attr.meta.require_list() {
                Ok(meta_list) => meta_list,
                Err(err) => return err.to_compile_error().into(),
            };

            let parser = Punctuated::<Type, Token![,]>::parse_terminated;
            let types = match parser.parse2(meta_list.tokens.clone()) {
                Ok(types) => types,
                Err(err) => return err.to_compile_error().into(),
            };

            if types.is_empty() {
                return syn::Error::new_spanned(
                    attr,
                    "#[from(...)] requires at least one source type",
                )
                .to_compile_error()
                .into();
            }

            for source_ty in types {
                let source_key = quote!(#source_ty).to_string();
                if !seen_sources.insert(source_key) {
                    return syn::Error::new_spanned(
                        attr,
                        "duplicate #[from(...)] source type on the same enum",
                    )
                    .to_compile_error()
                    .into();
                }

                generated_impls.push(quote! {
                    impl #impl_generics core::convert::From<#source_ty> for #enum_name #ty_generics #where_clause {
                        fn from(_error: #source_ty) -> Self {
                            Self::#variant_name
                        }
                    }
                });
            }
        }
    }

    quote! {
        #(#generated_impls)*
    }
    .into()
}
