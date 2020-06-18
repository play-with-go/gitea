bashExport() {
	echo export "$1=$2"
}

bashAlias() {
	echo alias "$1=$2"
}

smartcdExport() {
	echo autostash "$1=$2"
}

smartcdAlias() {
	echo autostash alias "$1=$2"
}

githubExport() {
	echo "::set-env name=$1::$2"
}

githubAlias() {
	# no-op
	echo ""
}

export=bashExport
alias=bashAlias

if [ "$1" = "smartcd" ]
then
	export="smartcdExport"
	alias="smartcdAlias"
elif [ "$1" = "github" ]
then
	export="githubExport"
	alias="githubAlias"
fi
