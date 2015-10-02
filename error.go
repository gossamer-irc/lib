package lib

type NameInUseError struct{}

func (_ NameInUseError) Error() string {
	return "NicknameInUse"
}

type AlreadyAMemberError struct{}

func (_ AlreadyAMemberError) Error() string {
	return "AlreadyAMember"
}
