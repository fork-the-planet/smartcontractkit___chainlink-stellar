use common_error::ErrorConversions;

#[derive(Debug)]
struct SourceA;

#[derive(Debug)]
struct SourceB;

#[derive(Debug, PartialEq, Eq, ErrorConversions)]
enum TargetError {
    #[from(SourceA)]
    Unauthorized,
    #[from(SourceB)]
    NotInitialized,
}

#[test]
fn derives_from_impl_for_annotated_variants() {
    assert_eq!(TargetError::from(SourceA), TargetError::Unauthorized);
    assert_eq!(TargetError::from(SourceB), TargetError::NotInitialized);
}
